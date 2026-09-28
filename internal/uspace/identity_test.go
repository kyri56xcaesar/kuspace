package uspace

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kyri56xcaesar/kuspace/internal/authn"
	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var (
	testJWTKey     = []byte("test-access-key")
	testRefreshKey = []byte("test-refresh-key")
)

func token(t *testing.T, key []byte, method jwt.SigningMethod, mutate func(*authn.Claims)) string {
	t.Helper()
	c := &authn.Claims{
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

// probe runs mw and returns the status and the Access-Target the handler saw.
func probe(t *testing.T, mw gin.HandlerFunc, headers map[string]string) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var seen string
	r.GET("/x", mw, func(c *gin.Context) { seen = c.GetHeader("Access-Target"); c.Status(http.StatusOK) })
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	return rec.Code, seen
}

func TestAuthenticate(t *testing.T) {
	srv := &UService{config: ut.EnvConfig{TokensConfig: ut.TokensConfig{JwtSecretKey: testJWTKey}, AuthConfig: ut.AuthConfig{ServiceSecretKey: []byte("svc")}}}
	user := "Bearer " + token(t, testJWTKey, jwt.SigningMethodHS256, nil)

	// a user claiming root in Access-Target gets their own identity instead
	code, at := probe(t, authenticate(srv), map[string]string{"Authorization": user, "Access-Target": ":vol:/f.txt 0:0"})
	if code != http.StatusOK || at != ":vol:/f.txt 1001:1002,1000" {
		t.Errorf("user token: %d %q", code, at)
	}
	// services may state the identity
	code, at = probe(t, authenticate(srv), map[string]string{"X-Service-Secret": "svc", "Access-Target": ":vol:/f.txt 0:0"})
	if code != http.StatusOK || at != ":vol:/f.txt 0:0" {
		t.Errorf("service: %d %q", code, at)
	}
	for name, h := range map[string]map[string]string{
		"nothing":       {"Access-Target": ":vol:/f 0:0"},
		"wrong secret":  {"X-Service-Secret": "nope"},
		"bad token":     {"Authorization": "Bearer nope"},
		"refresh token": {"Authorization": "Bearer " + token(t, testRefreshKey, jwt.SigningMethodHS256, nil)},
	} {
		if code, _ := probe(t, authenticate(srv), h); code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, code)
		}
	}

	// admin routes: services and the admin group only
	if code, _ := probe(t, authenticateAdmin(srv), map[string]string{"Authorization": user}); code != http.StatusForbidden {
		t.Errorf("non-admin on admin route: %d", code)
	}
	admin := "Bearer " + token(t, testJWTKey, jwt.SigningMethodHS256, func(c *authn.Claims) { c.Groups = "admin,user" })
	if code, _ := probe(t, authenticateAdmin(srv), map[string]string{"Authorization": admin}); code != http.StatusOK {
		t.Errorf("admin token on admin route: %d", code)
	}
	if code, _ := probe(t, authenticateAdmin(srv), map[string]string{"X-Service-Secret": "svc"}); code != http.StatusOK {
		t.Errorf("service on admin route: %d", code)
	}
}
