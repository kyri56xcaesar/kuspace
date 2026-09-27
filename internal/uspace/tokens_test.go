package uspace

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/golang-jwt/jwt/v5"
)

func rsaToken(t *testing.T, key *rsa.PrivateKey, kid string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, &accessClaims{
		UserID: "1001", GroupIDs: "1000,1002", PGroup: "1002",
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "minioth", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	return s
}

// jwksServer serves the public halves of keys (kid -> key) as minioth does.
func jwksServer(t *testing.T, keys map[string]*rsa.PrivateKey, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		var set struct {
			Keys []map[string]string `json:"keys"`
		}
		for kid, k := range keys {
			set.Keys = append(set.Keys, map[string]string{
				"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig",
				"n": base64.RawURLEncoding.EncodeToString(k.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes()),
			})
		}
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestTokenVerifierRS256(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	var hits atomic.Int32
	keys := map[string]*rsa.PrivateKey{"k1": key}
	js := jwksServer(t, keys, &hits)

	v := newTokenVerifier(ut.EnvConfig{JwtSigningAlg: "RS256", JwtSecretKey: []byte("hs-key")})
	v.jwksURL = js.URL

	if c, err := v.verify(rsaToken(t, key, "k1")); err != nil || c.UserID != "1001" {
		t.Fatalf("valid RS256 token rejected: %v", err)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("JWKS fetched %d times, want 1 (cached)", n)
	}

	// algorithm confusion: an HS256 token "signed" with the RSA public key
	pub, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	confused, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, &accessClaims{
		UserID: "0", RegisteredClaims: jwt.RegisteredClaims{Issuer: "minioth", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString(pub)
	if _, err := v.verify(confused); err == nil {
		t.Error("HS256 token accepted by an RS256 verifier (algorithm confusion)")
	}
	// the shared HS256 key is not accepted either when RS256 is configured
	if _, err := v.verify(token(t, []byte("hs-key"), jwt.SigningMethodHS256, nil)); err == nil {
		t.Error("HS256 token accepted by an RS256 verifier")
	}

	// unknown key ids don't refetch on every request
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	for range 5 {
		if _, err := v.verify(rsaToken(t, other, "k2")); err == nil {
			t.Fatal("token from an unknown key accepted")
		}
	}
	if n := hits.Load(); n > 2 {
		t.Errorf("JWKS fetched %d times for unknown kids, want at most one refetch", n)
	}

	// key rotation: once the interval passes, a new kid is picked up
	keys["k2"] = other
	v.lastFetch = time.Now().Add(-2 * jwksRefetchInterval)
	if _, err := v.verify(rsaToken(t, other, "k2")); err != nil {
		t.Errorf("rotated key not picked up: %v", err)
	}
}

func TestTokenVerifierHS256RejectsRS256(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newTokenVerifier(ut.EnvConfig{JwtSecretKey: testJWTKey})
	if _, err := v.verify(rsaToken(t, key, "k1")); err == nil {
		t.Error("RS256 token accepted by an HS256 verifier")
	}
}
