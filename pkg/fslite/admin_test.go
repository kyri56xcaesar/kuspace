package fslite

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAdminTokens(t *testing.T) {
	old := jwtSecretKey
	t.Cleanup(func() { jwtSecretKey = old })

	jwtSecretKey = nil
	if _, err := generateAccessJWT("id", "admin"); err == nil {
		t.Fatal("issued a token without a configured key")
	}

	jwtSecretKey = deriveTokenKey([]byte("configured-secret"))
	tok, err := generateAccessJWT("id-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if ok, c, err := decodeJWT(tok); !ok || err != nil || c.Username != "admin" {
		t.Fatalf("own token rejected: %v", err)
	}

	claims := CustomClaims{ID: "id-0", Username: "attacker", RegisteredClaims: jwt.RegisteredClaims{
		Issuer: "fslite", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	for name, key := range map[string][]byte{
		"the old hardcoded key": []byte("r4nd0m"),
		"the raw JWT secret":    []byte("configured-secret"), // fslite's key is derived, never the raw one
	} {
		forged, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
		if ok, _, err := decodeJWT(forged); ok || err == nil {
			t.Errorf("token signed with %s accepted", name)
		}
	}
	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(jwtSecretKey)
	if ok, _, _ := decodeJWT(hs512); ok {
		t.Error("HS512 token accepted (only HS256 is issued)")
	}
}

func TestAdminPasswordRules(t *testing.T) {
	for pw, ok := range map[string]bool{
		"short1":                  false,
		"with-dash-and.dot":       true, // used to be rejected
		"has space in it":         false,
		string(make([]byte, 73)):  false, // beyond bcrypt's limit (and not printable)
		"Aa1!Aa1!Aa1!Aa1!Aa1!xyz": true,
	} {
		err := (&Admin{Username: "admin", Password: pw}).validate()
		if (err == nil) != ok {
			t.Errorf("password %q: err=%v, want ok=%v", pw, err, ok)
		}
	}
}

func TestTokenValidity(t *testing.T) {
	old := JwtValidityHours
	t.Cleanup(func() { JwtValidityHours = old })
	for hours, want := range map[float64]time.Duration{0.5: 30 * time.Minute, 2: 2 * time.Hour, 0: 4 * time.Hour} {
		JwtValidityHours = hours
		if got := tokenValidity(); got != want {
			t.Errorf("%v hours -> %v, want %v", hours, got, want)
		}
	}
}
