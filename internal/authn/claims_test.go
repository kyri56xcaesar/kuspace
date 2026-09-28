package authn

import (
	"testing"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/golang-jwt/jwt/v5"
)

var (
	testJWTKey     = []byte("test-access-key")
	testRefreshKey = []byte("test-refresh-key")
)

func token(t *testing.T, key []byte, method jwt.SigningMethod, mutate func(*Claims)) string {
	t.Helper()
	c := &Claims{
		UserID: "1001", Username: "alice", Groups: "user,alice", GroupIDs: "1000,1002", PGroup: "1002",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "minioth",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	if mutate != nil {
		mutate(c)
	}
	s, err := jwt.NewWithClaims(method, c).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	return s
}

func TestVerifyAccessToken(t *testing.T) {
	hs := NewVerifier(ut.EnvConfig{TokensConfig: ut.TokensConfig{JwtSecretKey: testJWTKey}}) // HS256 is the default
	good := token(t, testJWTKey, jwt.SigningMethodHS256, nil)
	if c, err := hs.Verify(good); err != nil || c.UserID != "1001" {
		t.Fatalf("valid token rejected: %v", err)
	}
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{UserID: "0"}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	for name, raw := range map[string]string{
		"refresh-key token": token(t, testRefreshKey, jwt.SigningMethodHS256, nil),
		"alg none":          none,
		"expired":           token(t, testJWTKey, jwt.SigningMethodHS256, func(c *Claims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute)) }),
		"no expiry":         token(t, testJWTKey, jwt.SigningMethodHS256, func(c *Claims) { c.ExpiresAt = nil }),
		"wrong issuer":      token(t, testJWTKey, jwt.SigningMethodHS256, func(c *Claims) { c.Issuer = "someone" }),
		"no user":           token(t, testJWTKey, jwt.SigningMethodHS256, func(c *Claims) { c.UserID = "" }),
		"garbage":           "not.a.jwt",
	} {
		if _, err := hs.Verify(raw); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestWhoPutsPrimaryGroupFirst(t *testing.T) {
	c := &Claims{UserID: "1001", GroupIDs: "1000,1002", PGroup: "1002"}
	if got := c.Who(); got != "1001:1002,1000" {
		t.Errorf("Who() = %q", got)
	}
	if got := (&Claims{UserID: "7"}).Who(); got != "7:7" {
		t.Errorf("Who() without groups = %q", got)
	}
}

// InGroup compares names exactly: frontapp used to test
// strings.Contains("user,admin", group), so a group named "adm" - or a
// token with no groups at all ("") - passed the admin check.
func TestInGroupIsExact(t *testing.T) {
	cases := []struct {
		groups string
		admin  bool
	}{
		{"user,admin", true},
		{" admin ", true},
		{"adm", false},
		{"min,user", false},
		{"", false},
		{",,", false},
		{"administrators", false},
	}
	for _, tc := range cases {
		if got := (&Claims{Groups: tc.groups}).IsAdmin(); got != tc.admin {
			t.Errorf("groups %q: admin = %v, want %v", tc.groups, got, tc.admin)
		}
	}
	if !(&Claims{Groups: "alice,user"}).InGroup("user", "admin") {
		t.Error("user not in user,admin")
	}
}
