package frontendapp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCSRF(t *testing.T) {
	h := newFrontHarness(t)
	h.uspace.on("DELETE /api/v1/resource/rm", http.StatusOK, `{"message":"ok"}`)
	user := login(t, "1001", "user")
	rm := "/api/v1/verified/rm?name=/a.txt"

	send := func(cookieToken, headerToken, origin string) int {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, rm, nil)
		req.Host = "kuspace.local"
		req.AddCookie(user)
		if cookieToken != "" {
			req.AddCookie(&http.Cookie{Name: csrfCookie, Value: cookieToken})
		}
		if headerToken != "" {
			req.Header.Set(csrfHeader, headerToken)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		h.srv.Engine.ServeHTTP(rec, req)

		return rec.Code
	}
	other := strings.Repeat("cd", 32)
	cases := []struct {
		name                   string
		cookie, header, origin string
		want                   int
	}{
		{"token in cookie and header", testCSRF, testCSRF, "http://kuspace.local", http.StatusOK},
		{"no origin (non-browser client)", testCSRF, testCSRF, "", http.StatusOK},
		{"no header (a forged form)", testCSRF, "", "", http.StatusForbidden},
		{"wrong header", testCSRF, other, "", http.StatusForbidden},
		{"no cookie", "", testCSRF, "", http.StatusForbidden},
		{"another site", testCSRF, testCSRF, "https://evil.example", http.StatusForbidden},
	}
	for _, tc := range cases {
		if got := send(tc.cookie, tc.header, tc.origin); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}

	// every page hands out a token the page's JS can read
	rec := h.do(http.MethodGet, "/api/v1/login", nil, nil, nil)
	c := rec.Header().Get("Set-Cookie")
	if !strings.Contains(c, csrfCookie+"=") || strings.Contains(c, "HttpOnly") || !strings.Contains(c, "SameSite=Strict") {
		t.Errorf("csrf cookie: %q", c)
	}

	// login needs no token (there is no page state yet), but not from another site
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/login", strings.NewReader("username=a&password=bbbb"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.srv.Engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-site login: %d, want 403", rec.Code)
	}
}

func TestLogoutClearsOnlyTheSession(t *testing.T) {
	h := newFrontHarness(t)
	rec := h.do(http.MethodDelete, "/api/v1/logout?csrf_token&other", login(t, "1001", "user"), nil, nil)
	cookies := rec.Header().Values("Set-Cookie")
	if rec.Code != http.StatusNoContent || rec.Header().Get("Hx-Redirect") != "/api/v1/login" {
		t.Errorf("logout: %d %v", rec.Code, rec.Header())
	}
	for _, c := range cookies {
		if !strings.HasPrefix(c, sessionCookie+"=;") {
			t.Errorf("logout touched %q", c)
		}
	}
}

func TestStrictCSP(t *testing.T) {
	h := newFrontHarness(t)
	csp := h.do(http.MethodGet, "/api/v1/login", nil, nil, nil).Header().Get("Content-Security-Policy")
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") || !strings.Contains(csp, "script-src 'self';") {
		t.Errorf("CSP = %q", csp)
	}
}
