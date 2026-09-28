package frontendapp

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

func TestJobCallsActAsTheUser(t *testing.T) {
	h := newFrontHarness(t)
	h.uspace.on("GET /api/v1/job", http.StatusOK, `{"content":[{"jid":1,"uid":1001,"createdAt":"2026-09-01 10:00:00+00:00"},{"jid":2,"uid":1001,"createdAt":"2026-09-02 10:00:00+00:00"}]}`)
	h.uspace.on("POST /api/v1/job", http.StatusOK, `{"jid":7,"status":"queued"}`)
	h.uspace.on("GET /api/v1/job/log", http.StatusOK, "the log")
	user := login(t, "1001", "user")

	rec := h.do(http.MethodGet, "/api/v1/verified/fetch-jobs?format=json", user, nil, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"admin":0`) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	got := h.uspace.last(t)
	if got.header.Get("Authorization") != "Bearer "+user.Value || got.header.Get("X-Service-Secret") != "" || got.query != "" {
		t.Errorf("list sent auth %q, secret %q, query %q: want the user's token and no uid filter (uspace scopes)",
			got.header.Get("Authorization"), got.header.Get("X-Service-Secret"), got.query)
	}
	if i, j := strings.Index(rec.Body.String(), `"jid":2`), strings.Index(rec.Body.String(), `"jid":1`); i < 0 || i > j {
		t.Error("jobs not newest first")
	}

	form := url.Values{"input": {"vol/in.csv"}, "output": {"vol/out.csv"}, "logic": {"bash"}, "logicBody": {"cat {input}"},
		"parallelism": {"1"}, "timeout": {"5"}, "uid": {"0"}}
	h.srv.Config.JobsConfig = ut.JobsConfig{UspaceJobMaxCPU: 4, UspaceJobMaxMemory: 4096, UspaceJobMaxStorage: 4,
		UspaceJobMaxParallelism: 4, UspaceJobMaxTimeout: 60, UspaceJobMaxLogicSize: 1000}
	rec = h.do(http.MethodPost, "/api/v1/verified/jobs", user, strings.NewReader(form.Encode()),
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"jid":7`) {
		t.Fatalf("submit: %d %s", rec.Code, rec.Body)
	}
	if body := h.uspace.last(t).body; !strings.Contains(body, `"uid":1001`) {
		t.Errorf("submitted job %s: the owner must be the session user, not the form's uid", body)
	}

	h.uspace.on("POST /api/v1/job", http.StatusForbidden, `{"error":"no read access to input vol/in.csv"}`)
	rec = h.do(http.MethodPost, "/api/v1/verified/jobs", user, strings.NewReader(form.Encode()),
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "no read access") {
		t.Errorf("refused job: %d %s", rec.Code, rec.Body)
	}

	rec = h.do(http.MethodGet, "/api/v1/verified/job-log?jid=7", user, nil, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "the log" || h.uspace.last(t).query != "jid=7" {
		t.Errorf("log: %d %q", rec.Code, rec.Body)
	}
	if rec := h.do(http.MethodGet, "/api/v1/verified/job-log?jid=x", user, nil, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("bad jid: %d", rec.Code)
	}
}

func TestWSTicketOnlyForOwnJobs(t *testing.T) {
	h := newFrontHarness(t)
	h.uspace.on("GET /api/v1/job", http.StatusOK, `{"content":{"jid":7,"uid":1001}}`)

	if rec := h.do(http.MethodGet, "/api/v1/verified/ws-ticket?jid=7", login(t, "1001", "user"), nil, nil); rec.Code != http.StatusOK {
		t.Errorf("owner: %d %s", rec.Code, rec.Body)
	}
	if got := h.uspace.last(t); got.header.Get("X-Service-Secret") != "svc" || got.query != "jids=7" {
		t.Errorf("owner lookup: secret %q query %q", got.header.Get("X-Service-Secret"), got.query)
	}
	if rec := h.do(http.MethodGet, "/api/v1/verified/ws-ticket?jid=7", login(t, "2002", "user"), nil, nil); rec.Code != http.StatusForbidden {
		t.Errorf("stranger: %d", rec.Code)
	}
	if rec := h.do(http.MethodGet, "/api/v1/verified/ws-ticket?jid=7", login(t, "1", "admin"), nil, nil); rec.Code != http.StatusOK {
		t.Errorf("admin: %d", rec.Code)
	}
}
