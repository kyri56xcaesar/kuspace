package frontendapp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func form(v url.Values) (*strings.Reader, map[string]string) {
	return strings.NewReader(v.Encode()), map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
}

func TestLogin(t *testing.T) {
	h := newFrontHarness(t)
	good := login(t, "1001", "user").Value
	h.minioth.on("POST /v1/login", http.StatusOK, `{"access_token":"`+good+`"}`)

	body, hdr := form(url.Values{"username": {"alice"}, "password": {"secret-pass"}})
	rec := h.do(http.MethodPost, "/api/v1/login", nil, body, hdr)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Set-Cookie"), sessionCookie+"="+good) {
		t.Fatalf("login: %d, cookie %q", rec.Code, rec.Header().Get("Set-Cookie"))
	}
	if c := rec.Header().Get("Set-Cookie"); !strings.Contains(c, "HttpOnly") || !strings.Contains(c, "SameSite=Strict") {
		t.Errorf("session cookie flags: %q", c)
	}

	// a token this frontapp can't verify is not turned into a session
	h.minioth.on("POST /v1/login", http.StatusOK, `{"access_token":"not-a-jwt"}`)
	body, hdr = form(url.Values{"username": {"alice"}, "password": {"secret-pass"}})
	if rec := h.do(http.MethodPost, "/api/v1/login", nil, body, hdr); rec.Code != http.StatusBadGateway || rec.Header().Get("Set-Cookie") != "" {
		t.Errorf("unverifiable token: %d, cookie %q", rec.Code, rec.Header().Get("Set-Cookie"))
	}

	h.minioth.on("POST /v1/login", http.StatusUnauthorized, `{"error":"invalid credentials"}`)
	body, hdr = form(url.Values{"username": {"alice"}, "password": {"wrong-pass"}})
	if rec := h.do(http.MethodPost, "/api/v1/login", nil, body, hdr); rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "invalid credentials") {
		t.Errorf("wrong password: %d %s", rec.Code, rec.Body)
	}
}

func TestNewUsersGetStorage(t *testing.T) {
	h := newFrontHarness(t)
	h.minioth.on("POST /v1/register", http.StatusOK, `{"uid":1005,"pgroup":1005}`)
	h.minioth.on("POST /v1/admin/useradd", http.StatusOK, `{"uid":1006,"pgroup":1006}`)
	h.uspace.on("POST /api/v1/admin/user/volume", http.StatusCreated, `{"status":"ok"}`)

	body, hdr := form(url.Values{"username": {"newbie"}, "password": {"secret-pass"}, "repeatPassword": {"secret-pass"}})
	if rec := h.do(http.MethodPost, "/api/v1/register", nil, body, hdr); rec.Code != http.StatusSeeOther {
		t.Fatalf("register: %d %s", rec.Code, rec.Body)
	}
	claim := h.uspace.last(t)
	if claim.path != "/api/v1/admin/user/volume" || !strings.Contains(claim.body, `"uid":1005`) || claim.header.Get("X-Service-Secret") != "svc" {
		t.Errorf("storage claim: %+v", claim)
	}

	// an admin adding a user gives them storage too (they used to get none until uspace restarted)
	body, hdr = form(url.Values{"username": {"bob"}, "password": {"secret-pass"}})
	if rec := h.do(http.MethodPost, "/api/v1/verified/admin/useradd", login(t, "1", "admin"), body, hdr); rec.Code != http.StatusOK {
		t.Fatalf("useradd: %d %s", rec.Code, rec.Body)
	}
	if claim := h.uspace.last(t); !strings.Contains(claim.body, `"uid":1006`) {
		t.Errorf("useradd storage claim: %+v", claim)
	}

	body, hdr = form(url.Values{"username": {"newbie"}, "password": {"secret-pass"}, "repeatPassword": {"other-pass"}})
	if rec := h.do(http.MethodPost, "/api/v1/register", nil, body, hdr); rec.Code != http.StatusBadRequest {
		t.Errorf("mismatched passwords: %d", rec.Code)
	}
}

func TestUserListNeverCarriesHashes(t *testing.T) {
	h := newFrontHarness(t)
	h.minioth.on("GET /v1/admin/users", http.StatusOK, `{"content":[{"uid":1,"username":"root","password":{"hashpass":"$2a$10$abcdefghijklmnopqrstuv"}}]}`)
	rec := h.do(http.MethodGet, "/api/v1/verified/admin/fetch-users?format=json", login(t, "1", "admin"), nil, nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "$2a$") {
		t.Errorf("users: %d %s", rec.Code, rec.Body)
	}
	if got := h.minioth.last(t); got.header.Get("Authorization") == "" {
		t.Error("admin call without the admin's token")
	}
}

func TestPasswordChangeEndsTheSession(t *testing.T) {
	h := newFrontHarness(t)
	session := login(t, "1001", "user")
	change := func(cur, next, repeat string) *httptest.ResponseRecorder {
		body, hdr := form(url.Values{"currentPassword": {cur}, "newPassword": {next}, "newPasswordRepeat": {repeat}})

		return h.do(http.MethodPost, "/api/v1/verified/passwd", session, body, hdr)
	}

	h.minioth.on("POST /v1/passwd", http.StatusOK, `{"message":"password changed"}`)
	rec := change("old-pass", "new-pass", "new-pass")
	if rec.Code != http.StatusNoContent || rec.Header().Get("Hx-Redirect") != "/api/v1/login" ||
		!strings.HasPrefix(rec.Header().Get("Set-Cookie"), sessionCookie+"=;") {
		t.Fatalf("password change: %d %v", rec.Code, rec.Header())
	}
	got := h.minioth.last(t)
	if got.header.Get("Authorization") != "Bearer "+session.Value || got.header.Get("X-Service-Secret") != "" ||
		got.body != `{"current_password":"old-pass","new_password":"new-pass"}` {
		t.Errorf("minioth got %+v", got)
	}

	// minioth checks the current password; its answer reaches the user and the session stays
	h.minioth.on("POST /v1/passwd", http.StatusUnauthorized, `{"error":"current password is wrong"}`)
	rec = change("bad-pass", "new-pass", "new-pass")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "current password is wrong") || rec.Header().Get("Set-Cookie") != "" {
		t.Errorf("wrong current password: %d %s %v", rec.Code, rec.Body, rec.Header())
	}

	before := len(h.minioth.seen)
	if rec := change("old-pass", "new-pass", "other-pass"); rec.Code != http.StatusBadRequest || len(h.minioth.seen) != before {
		t.Errorf("mismatched repeat: %d", rec.Code)
	}
}

func TestEmailChangeIsSelfService(t *testing.T) {
	h := newFrontHarness(t)
	h.minioth.on("PATCH /v1/user/me", http.StatusOK, `{"message":"updated"}`)
	session := login(t, "1001", "user")
	body, hdr := form(url.Values{"new-email-change": {"a@example.com"}})
	if rec := h.do(http.MethodPut, "/api/v1/verified/user-update", session, body, hdr); rec.Code != http.StatusOK {
		t.Fatalf("email change: %d %s", rec.Code, rec.Body)
	}
	got := h.minioth.last(t)
	if got.header.Get("Authorization") != "Bearer "+session.Value || got.header.Get("X-Service-Secret") != "" ||
		!strings.Contains(got.body, `"email":"a@example.com"`) {
		t.Errorf("minioth got %+v", got)
	}
}
