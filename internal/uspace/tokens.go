package uspace

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/golang-jwt/jwt/v5"
)

// jwksRefetchInterval limits how often an unknown key id triggers a JWKS
// fetch (key rotation), so forged tokens can't hammer minioth.
const jwksRefetchInterval = 30 * time.Second

// tokenVerifier checks minioth access tokens with the algorithm uspace is
// configured for - never the one a token names in its header (accepting
// the header's choice lets an attacker sign HS256 tokens with the RS256
// public key as the "secret").
//
//	HS256: the shared JWT_SECRET_KEY (refresh and purpose tokens use
//	       another key, so they fail here)
//	RS256: minioth's public keys from its JWKS, cached by key id
type tokenVerifier struct {
	alg     string
	hmacKey []byte
	jwksURL string
	client  *http.Client

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	lastFetch time.Time
}

func newTokenVerifier(cfg ut.EnvConfig) *tokenVerifier {
	alg := strings.ToUpper(strings.TrimSpace(cfg.JwtSigningAlg))
	if alg == "" {
		alg = jwt.SigningMethodHS256.Alg()
	}

	return &tokenVerifier{
		alg:     alg,
		hmacKey: cfg.JwtSecretKey,
		jwksURL: "http://" + cfg.AuthAddress + ":" + cfg.AuthPort + "/v1/.well-known/jwks.json",
		client:  &http.Client{Timeout: 5 * time.Second},
		keys:    map[string]*rsa.PublicKey{},
	}
}

// verify validates raw as a minioth access token: the configured algorithm,
// issuer "minioth", an expiry, and a user id.
func (v *tokenVerifier) verify(raw string) (*accessClaims, error) {
	claims := &accessClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, v.key,
		jwt.WithValidMethods([]string{v.alg}),
		jwt.WithIssuer("minioth"),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	if claims.UserID == "" {
		return nil, errors.New("token has no user_id")
	}

	return claims, nil
}

func (v *tokenVerifier) key(t *jwt.Token) (any, error) {
	switch v.alg {
	case "HS256":
		if len(v.hmacKey) == 0 {
			return nil, errors.New("no JWT secret configured")
		}

		return v.hmacKey, nil
	case "RS256":
		kid, _ := t.Header["kid"].(string)

		return v.rsaKey(kid)
	default:
		return nil, fmt.Errorf("unsupported JWT_SIGNING_ALG %q", v.alg)
	}
}

// rsaKey returns minioth's public key kid, fetching the JWKS when the key
// isn't cached yet (at most every jwksRefetchInterval).
func (v *tokenVerifier) rsaKey(kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	if time.Since(v.lastFetch) < jwksRefetchInterval {
		return nil, fmt.Errorf("unknown signing key %q", kid)
	}
	v.lastFetch = time.Now()
	keys, err := v.fetchJWKS()
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	v.keys = keys
	if k, ok := keys[kid]; ok {
		return k, nil
	}

	return nil, fmt.Errorf("unknown signing key %q", kid)
}

func (v *tokenVerifier) fetchJWKS() (map[string]*rsa.PublicKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("minioth answered %s", resp.Status)
	}
	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil || len(e) == 0 {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}

	return keys, nil
}
