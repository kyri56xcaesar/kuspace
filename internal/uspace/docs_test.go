package uspace

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"kyri56xcaesar/kuspace/internal/authn"
	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/golang-jwt/jwt/v5"
)

var routerAnnotation = regexp.MustCompile(`(?m)^//\s*@Router\s+(\S+)\s+\[(\w+)\]`)

// documented returns "METHOD path" for every @Router annotation in dir.
func documented(t *testing.T, dir, base string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var routes []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range routerAnnotation.FindAllStringSubmatch(string(src), -1) {
			routes = append(routes, strings.ToUpper(m[2])+" "+base+m[1])
		}
	}
	slices.Sort(routes)

	return slices.Compact(routes)
}

// The API docs are generated from the @Router annotations (make api-docs):
// every annotation must name a registered route and every API route must be
// annotated. They had drifted: methods changed, admin routes documented
// under the user API, a documented method that wasn't routed.
func TestAPIDocsMatchTheRoutes(t *testing.T) {
	a := newAPIHarness(t, 1)
	var registered []string
	for _, r := range a.srv.Engine.Routes() {
		if strings.HasPrefix(r.Path, "/api/v1/") && !strings.HasPrefix(r.Path, "/api/v1/swagger") {
			registered = append(registered, r.Method+" "+r.Path)
		}
	}
	docs := documented(t, ".", "/api/v1")
	for _, d := range docs {
		if !slices.Contains(registered, d) {
			t.Errorf("documented, not routed: %s", d)
		}
	}
	for _, r := range registered {
		if !slices.Contains(docs, r) {
			t.Errorf("routed, not documented: %s", r)
		}
	}
}

// GET and POST /admin/job were handled but not routed.
func TestAdminJobRoutes(t *testing.T) {
	a := newAPIHarness(t, 1)
	a.withJobsDB(1001, 2002, 2002)
	rec := a.do(http.MethodGet, "/api/v1/admin/job", "::/", "0:0", nil, "")
	var body struct {
		Content []ut.Job `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); rec.Code != http.StatusOK || err != nil || len(body.Content) != 3 {
		t.Errorf("admin job listing: %d %s", rec.Code, rec.Body)
	}
	// a user's own token is not enough (the service secret is trusted: frontapp checks roles)
	a.srv.config.JwtSecretKey = []byte("jwt")
	a.srv.tokens = nil
	claims := &authn.Claims{UserID: "1001", Groups: "user", GroupIDs: "1001",
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "minioth", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("jwt"))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/admin/job", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec = httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusUnauthorized {
		t.Errorf("a user on the admin job API: %d", rec.Code)
	}
}
