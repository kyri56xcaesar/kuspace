package frontendapp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"kyri56xcaesar/kuspace/internal/authn"
	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var testKey = []byte("frontapp-test-key")

// seen is one request an upstream fake received.
type seen struct {
	method, path, query string
	header              http.Header
	body                string
}

// fakeUpstream records requests and answers from a route table
// ("METHOD /path" -> handler); unknown routes are 404.
type fakeUpstream struct {
	*httptest.Server
	mu     sync.Mutex
	seen   []seen
	routes map[string]http.HandlerFunc
}

func newFakeUpstream(t *testing.T) *fakeUpstream {
	t.Helper()
	f := &fakeUpstream{routes: map[string]http.HandlerFunc{}}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.seen = append(f.seen, seen{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone(), string(b)})
		h := f.routes[r.Method+" "+r.URL.Path]
		f.mu.Unlock()
		if h == nil {
			http.Error(w, `{"error":"no such route in the fake"}`, http.StatusNotFound)

			return
		}
		h(w, r)
	}))
	t.Cleanup(f.Close)

	return f
}

func (f *fakeUpstream) on(route string, status int, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[route] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func (f *fakeUpstream) last(t *testing.T) seen {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.seen) == 0 {
		t.Fatal("the upstream got no request")
	}

	return f.seen[len(f.seen)-1]
}

type frontHarness struct {
	t               *testing.T
	srv             *HTTPService
	uspace, minioth *fakeUpstream
}

func newFrontHarness(t *testing.T) *frontHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	us, mo := newFakeUpstream(t), newFakeUpstream(t)
	cfg := ut.EnvConfig{
		AuthConfig:    ut.AuthConfig{JwtSecretKey: testKey, ServiceSecretKey: []byte("svc")},
		MinioConfig:   ut.MinioConfig{MinioDefaultBucket: "uspace-default"},
		ServiceConfig: ut.ServiceConfig{AllowedOrigins: []string{"*"}},
	}
	srv := &HTTPService{
		Engine:   gin.New(),
		Config:   cfg,
		webRoot:  "../..",
		verifier: authn.NewVerifier(cfg),
		uspace:   newUpstream(us.URL, cfg.ServiceSecretKey),
		minioth:  newUpstream(mo.URL, cfg.ServiceSecretKey),
		wss:      newUpstream("http://127.0.0.1:1", cfg.ServiceSecretKey),
	}
	srv.routes()

	return &frontHarness{t: t, srv: srv, uspace: us, minioth: mo}
}

// login returns a session cookie for a user in groups (names).
func login(t *testing.T, uid, groups string, mutate ...func(*authn.Claims)) *http.Cookie {
	t.Helper()
	c := &authn.Claims{
		UserID: uid, Username: "user" + uid, Groups: groups, GroupIDs: uid, PGroup: uid,
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "minioth", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}
	for _, m := range mutate {
		m(c)
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(testKey)
	if err != nil {
		t.Fatal(err)
	}

	return &http.Cookie{Name: sessionCookie, Value: s}
}

func (h *frontHarness) do(method, path string, cookie *http.Cookie, body io.Reader, header map[string]string) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequestWithContext(h.t.Context(), method, path, body)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.srv.Engine.ServeHTTP(rec, req)

	return rec
}

func TestSessions(t *testing.T) {
	h := newFrontHarness(t)
	h.minioth.on("GET /v1/admin/users", http.StatusOK, `{"content":[]}`)
	users := "/api/v1/verified/admin/fetch-users?format=json"

	cases := []struct {
		name   string
		cookie *http.Cookie
		want   int
	}{
		{"admin", login(t, "1", "admin,user"), http.StatusOK},
		{"plain user on an admin page", login(t, "1001", "user"), http.StatusForbidden},
		// the old check was strings.Contains("admin", group)
		{"group named adm", login(t, "1002", "adm"), http.StatusForbidden},
		{"no groups at all", login(t, "1003", ""), http.StatusForbidden},
		{"no session", nil, http.StatusUnauthorized},
		{"expired", login(t, "1", "admin", func(c *authn.Claims) {
			c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
		}), http.StatusUnauthorized},
		{"forged", &http.Cookie{Name: sessionCookie, Value: func() string {
			s, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, &authn.Claims{UserID: "1", Groups: "admin",
				RegisteredClaims: jwt.RegisteredClaims{Issuer: "minioth", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).
				SignedString([]byte("not-the-key"))

			return s
		}()}, http.StatusUnauthorized},
	}
	for _, tc := range cases {
		if rec := h.do(http.MethodGet, users, tc.cookie, nil, nil); rec.Code != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, rec.Code, tc.want)
		}
	}

	// an expired session on a page goes to the login page, on htmx via HX-Redirect
	rec := h.do(http.MethodGet, "/api/v1/verified/admin-panel", nil, nil, map[string]string{"Accept": "text/html"})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/api/v1/login" {
		t.Errorf("page without a session: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = h.do(http.MethodGet, "/api/v1/verified/fetch-jobs", nil, nil, map[string]string{"HX-Request": "true"})
	if rec.Header().Get("Hx-Redirect") != "/api/v1/login" {
		t.Errorf("htmx without a session: %d, HX-Redirect %q", rec.Code, rec.Header().Get("Hx-Redirect"))
	}

	// the login page sends a logged-in user to the panel
	rec = h.do(http.MethodGet, "/api/v1/login", login(t, "1001", "user"), nil, nil)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "admin-panel") {
		t.Errorf("login page with a session: %d %q", rec.Code, rec.Header().Get("Location"))
	}
}
