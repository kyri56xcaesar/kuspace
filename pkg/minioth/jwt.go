// Package minioth provides JWT (JSON Web Token) utilities for authentication and authorization,
// supporting both HS256 (HMAC) and RS256 (RSA) signing methods. It includes functionality for
// generating, parsing, and validating JWTs, as well as managing RSA key rotation and JWKS (JSON Web Key Set)
// for public key distribution. The package defines custom claims, handles key storage and rotation,
// and supports both symmetric and asymmetric JWT workflows.
package minioth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	signingKeys      = map[string]RSAKey{}
	currentKID       string
	issuer                   = "minioth"
	jwtSecretKey             = []byte("default_placeholder_key")
	jwksFilePath             = "data/jwks/jwks.json"
	jwtValidityHours float64 = 1
	rsaKeyTTL        float64 = 1
)

// JWK struct includes data that describe a key
type JWK struct {
	Kty string    `json:"kty"` // Key Type
	Alg string    `json:"alg"` // Algorithm
	Use string    `json:"use"` // Public Key Use
	Kid string    `json:"kid"` // Key ID
	N   string    `json:"n"`   // Modulus
	E   string    `json:"e"`   // Exponent
	TTL time.Time `json:"ttl"`
}

func loadJWKS() (*JWKS, error) {
	data, err := os.ReadFile(jwksFilePath) // or fetch from URL if remote
	if err != nil {
		return nil, fmt.Errorf("failed to read JWKS file: %w", err)
	}

	var jwks JWKS
	if err := json.Unmarshal(data, &jwks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWKS: %w", err)
	}

	return &jwks, nil
}

// JWKS struct holds a slice of JWT keys
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// RSAKey struct to describe a RSA key
type RSAKey struct {
	KID        string
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	CreatedAt  time.Time // TTL two hours
}

// CustomClaims struct includes extra data in the jwt load
type CustomClaims struct {
	Username string `json:"username"`
	Groups   string `json:"groups"`
	GroupIDs string `json:"groupIDs"`
	jwt.RegisteredClaims
}

// GenerateAccessHS256JWT function generates a jwt token signed with HS256 algorithm
func GenerateAccessHS256JWT(userID, username, groups, gids string) (string, error) {
	// Set the claims for the token
	claims := CustomClaims{
		Username: username,
		Groups:   groups,
		GroupIDs: gids,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * time.Duration(jwtValidityHours))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}

	// Create the token using the HS256 signing method
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token using the secret key
	tokenString, err := token.SignedString(jwtSecretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// GenerateAccessRS256JWT generates a JWT access token signed with the HS256 (HMAC-SHA256) algorithm.
// It accepts the user ID, username, groups, and group IDs as input, and embeds them as custom claims
// in the token payload. The token is issued by the configured issuer, has a configurable validity period,
// and is signed using the internal symmetric secret key. Returns the signed JWT string or an error if signing fails.
func GenerateAccessRS256JWT(userID, username, groups, gids string) (string, error) {
	claims := CustomClaims{
		Username: username,
		Groups:   groups,
		GroupIDs: gids,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * time.Duration(jwtValidityHours))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}

	key := signingKeys[currentKID]
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = key.KID

	return token.SignedString(key.PrivateKey)
}

// DecodeJWT .. d
func DecodeJWT(tokenString string) (bool, *CustomClaims, error) {
	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return jwtSecretKey, nil
	})
	if err != nil {
		log.Printf("%v token, exiting", token)

		return false, nil, err
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		log.Printf("not okay when retrieving claims")

		return false, nil, errors.New("invalid claims")
	}

	return true, claims, nil
}

// ParseJWT will parse properly a string to a jwt token object
func ParseJWT(tokenStr string) (*jwt.Token, error) {
	return jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		alg, ok := token.Header["alg"].(string)
		if !ok {
			return nil, errors.New("invalid token header (missing alg)")
		}

		switch alg {
		case "HS256":
			return jwtSecretKey, nil // your internal symmetric key
		case "RS256":
			// load your RSA public key (or use JWKS cache)
			return getRSAPublicKey(token, true) // validate via kid or static
		default:
			return nil, fmt.Errorf("unsupported signing method: %s", alg)
		}
	})
}

func rotateKey() {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("failed to generate RSA key: %v", err)
	}

	kid := uuid.New().String()
	signingKeys[kid] = RSAKey{
		KID:        kid,
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
		CreatedAt:  time.Now(),
	}
	currentKID = kid
	log.Printf("[INIT]rotated key, new kid: %s", kid)

	// perform a cleanup prune of old keys
	for kid, key := range signingKeys {
		if time.Since(key.CreatedAt) > time.Duration(rsaKeyTTL)*time.Hour {
			delete(signingKeys, kid)
		}
	}

	if err := updateJWKS(); err != nil {
		log.Fatalf("failed to update JWKS...: %v", err)
	}
}

func updateJWKS() error {
	var jwks JWKS
	for kid, key := range signingKeys {
		n := base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes())

		newJWK := JWK{
			Kty: "RSA",
			Alg: "RS256",
			Use: "sig",
			Kid: kid,
			N:   n,
			E:   e,
			TTL: key.CreatedAt.Add(time.Duration(rsaKeyTTL) * time.Hour),
		}
		// Append the new key
		jwks.Keys = append(jwks.Keys, newJWK)
	}
	data, err := json.MarshalIndent(jwks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal jwks: %w", err)
	}

	return os.WriteFile(jwksFilePath, data, 0o644)
}

func getRSAPublicKey(token *jwt.Token, static bool) (any, error) {
	if static {
		// Use locally stored RSA public key (e.g., loaded at server start)

		return signingKeys[currentKID].PublicKey, nil
	}

	// Get kid from token header
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, errors.New("no kid in token header")
	}

	// Look up JWKs (could be cached in memory or fetched from remote)
	jwks, err := loadJWKS()
	if err != nil {
		return nil, fmt.Errorf("failed to load JWKS: %w", err)
	}

	// Match by kid
	for _, key := range jwks.Keys {
		if key.Kid == kid {
			pubKey, err := keyToRSAPublicKey(key)
			if err != nil {
				return nil, fmt.Errorf("failed to parse RSA key from JWK: %w", err)
			}

			return pubKey, nil
		}
	}

	return nil, fmt.Errorf("no matching key for kid: %s", kid)
}

func keyToRSAPublicKey(jwk JWK) (*rsa.PublicKey, error) {
	if jwk.Kty != "RSA" {
		return nil, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}

	// Decode base64url encoded values
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert exponent to int
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	n := new(big.Int).SetBytes(nBytes)

	pubKey := &rsa.PublicKey{
		N: n,
		E: e,
	}

	return pubKey, nil
}
