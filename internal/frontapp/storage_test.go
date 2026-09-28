package frontendapp

import (
	"net/http"
	"strings"
	"testing"
)

// Storage calls carry the user's token and only the "what" of
// Access-Target: uspace takes the identity from the token. frontapp used
// to send the service secret plus an identity it assembled itself.
func TestStorageCallsActAsTheUser(t *testing.T) {
	h := newFrontHarness(t)
	h.uspace.on("GET /api/v1/resource/download", http.StatusOK, "file-bytes")
	h.uspace.on("DELETE /api/v1/resource/rm", http.StatusOK, `{"message":"ok"}`)
	h.uspace.on("PATCH /api/v1/resource/mv", http.StatusOK, `{"message":"ok"}`)
	user := login(t, "1001", "user")

	rec := h.do(http.MethodGet, "/api/v1/verified/download?volume=vol&target=/a%20b.txt", user, nil, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "file-bytes" {
		t.Fatalf("download: %d %q", rec.Code, rec.Body)
	}
	got := h.uspace.last(t)
	if got.header.Get("Authorization") != "Bearer "+user.Value {
		t.Error("the user's token was not forwarded")
	}
	if got.header.Get("X-Service-Secret") != "" {
		t.Error("a user call carried the service secret")
	}
	if at := got.header.Get("Access-Target"); at != "0:vol:/a b.txt" {
		t.Errorf("Access-Target = %q, want only the target (no identity)", at)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="a b.txt"` {
		t.Errorf("Content-Disposition = %q", cd)
	}

	// a name trying to smuggle an identity or a header stays inside the target
	h.do(http.MethodDelete, "/api/v1/verified/rm?volume=vol&name=/x.txt%200:0", user, nil, nil)
	if at := h.uspace.last(t).header.Get("Access-Target"); at != "0:vol:/x.txt 0:0" {
		t.Errorf("Access-Target = %q", at)
	}

	// the move destination is a query value, escaped (it used to be pasted in)
	h.do(http.MethodPatch, "/api/v1/verified/mv?volume=vol&resourcename=/a.txt", user,
		strings.NewReader("resourcename=b.txt"), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if q := h.uspace.last(t).query; q != "dest=vol%2Fb.txt" {
		t.Errorf("mv query = %q", q)
	}

	// upstream errors keep their status and message
	h.uspace.on("DELETE /api/v1/resource/rm", http.StatusForbidden, `{"error":"no write access"}`)
	rec = h.do(http.MethodDelete, "/api/v1/verified/rm?name=/x.txt", user, nil, nil)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "no write access") {
		t.Errorf("relayed error: %d %s", rec.Code, rec.Body)
	}
}

func TestFetchVolumesByRole(t *testing.T) {
	h := newFrontHarness(t)
	h.uspace.on("GET /api/v1/admin/volumes", http.StatusOK, `{"content":[{"vid":1,"name":"uspace-default"},{"vid":2,"name":"secret-project"}]}`)
	h.uspace.on("GET /api/v1/volumes/shared", http.StatusOK, `[{"vid":3,"vname":"team","gid":500,"quota":2,"usage":1}]`)

	rec := h.do(http.MethodGet, "/api/v1/verified/fetch-volumes?format=json", login(t, "1001", "user"), nil, nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "team") || !strings.Contains(body, "uspace-default") || strings.Contains(body, "secret-project") {
		t.Errorf("user's volumes: %d %s", rec.Code, body)
	}
	if h.uspace.last(t).path != "/api/v1/volumes/shared" {
		t.Errorf("user listing hit %s", h.uspace.last(t).path)
	}

	rec = h.do(http.MethodGet, "/api/v1/verified/fetch-volumes?format=json", login(t, "1", "admin"), nil, nil)
	if !strings.Contains(rec.Body.String(), "secret-project") {
		t.Errorf("admin's volumes: %s", rec.Body)
	}
}

// A service that can't be reached is a 503 with Retry-After; page loads
// get the error page.
func TestUpstreamDown(t *testing.T) {
	h := newFrontHarness(t)
	h.uspace.Close() // uspace is down
	user := login(t, "1001", "user")

	rec := h.do(http.MethodGet, "/api/v1/verified/fetch-jobs?format=json", user, nil, nil)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Errorf("API call: %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	rec = h.do(http.MethodGet, "/api/v1/verified/fetch-volumes", user, nil, map[string]string{"Accept": "text/html"})
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "not answering") {
		t.Errorf("page load: %d %.200s", rec.Code, rec.Body)
	}
	rec = h.do(http.MethodGet, "/readyz", nil, nil, nil)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"uspace"`) {
		t.Errorf("readyz: %d %s", rec.Code, rec.Body)
	}
}

func TestQuotaAdminRoutes(t *testing.T) {
	h := newFrontHarness(t)
	h.uspace.on("PATCH /api/v1/admin/user/volume", http.StatusOK, `{"vname":"vol1","uid":1001,"quota":5}`)
	h.uspace.on("POST /api/v1/admin/group/volume", http.StatusOK, `{"vname":"team","gid":500,"quota":2}`)
	h.uspace.on("DELETE /api/v1/admin/group/volume", http.StatusConflict, `{"error":"volume holds files (not empty)"}`)
	admin := login(t, "1", "admin")
	form := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}

	rec := h.do(http.MethodPatch, "/api/v1/verified/admin/user-volumes", admin, strings.NewReader("vname=vol1&uid=1001&quota=5"), form)
	if rec.Code != http.StatusOK || h.uspace.last(t).body != `{"quota":5,"uid":1001,"vname":"vol1"}` {
		t.Errorf("set quota: %d, sent %s", rec.Code, h.uspace.last(t).body)
	}
	rec = h.do(http.MethodPost, "/api/v1/verified/admin/group-volumes", admin, strings.NewReader("vname=team&gid=500&quota=2"), form)
	if rec.Code != http.StatusOK || !strings.Contains(h.uspace.last(t).body, `"gid":500`) {
		t.Errorf("share: %d, sent %s", rec.Code, h.uspace.last(t).body)
	}
	if rec := h.do(http.MethodDelete, "/api/v1/verified/admin/group-volumes?volume=team", admin, nil, nil); rec.Code != http.StatusConflict {
		t.Errorf("stop sharing a volume with files: %d", rec.Code)
	}
	if rec := h.do(http.MethodPost, "/api/v1/verified/admin/group-volumes", admin, strings.NewReader("vname=team"), form); rec.Code != http.StatusBadRequest {
		t.Errorf("missing gid: %d", rec.Code)
	}
	if rec := h.do(http.MethodGet, "/api/v1/verified/admin/user-volumes", login(t, "1001", "user"), nil, nil); rec.Code != http.StatusForbidden {
		t.Errorf("plain user: %d", rec.Code)
	}
}
