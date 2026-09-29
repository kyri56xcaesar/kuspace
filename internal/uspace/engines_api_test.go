package uspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kyri56xcaesar/kuspace/internal/model"
	"kyri56xcaesar/kuspace/internal/uspace/jobs"
)

func TestEngineSwitching(t *testing.T) {
	a := newAPIHarness(t, 1)
	a.withJobsDB(1001, 2002, 2002)
	if err := a.srv.jobs.MarkEngine(t.Context(), 1, "docker"); err != nil {
		t.Fatal(err)
	}
	k8s := &stubEngine{name: "kubernetes", err: "no cluster configuration"}
	a.srv.jdp = jobs.NewWithEngines(jobs.Deps{Store: a.srv.jobs}, &stubEngine{name: "docker"}, k8s)
	get := func() model.EnginesStatus {
		t.Helper()
		rec := a.do(http.MethodGet, "/api/v1/admin/executor", "::/", "0:0", nil, "")
		var st model.EnginesStatus
		if err := json.Unmarshal(rec.Body.Bytes(), &st); rec.Code != http.StatusOK || err != nil {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}

		return st
	}
	put := func(engine string) *httptest.ResponseRecorder {
		return a.do(http.MethodPut, "/api/v1/admin/executor", "::/", "0:0", strings.NewReader(`{"engine":"`+engine+`"}`), "application/json")
	}

	st := get()
	if st.Current != "docker" || len(st.Engines) != 2 || !st.Engines[0].Current || !st.Engines[0].Active ||
		st.Engines[1].Engine != "kubernetes" || st.Engines[1].Active || st.JobsByEngine["docker"] != 1 || st.ChosenBy != "" {
		t.Fatalf("status: %+v", st)
	}

	// an engine that doesn't answer can't take jobs; the reason is given
	if rec := put("kubernetes"); rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "no cluster configuration") {
		t.Errorf("switch to an inactive engine: %d %s", rec.Code, rec.Body)
	}
	if rec := put("nomad"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown engine: %d", rec.Code)
	}

	k8s.err = ""
	if rec := put("k8s"); rec.Code != http.StatusOK {
		t.Fatalf("switch: %d %s", rec.Code, rec.Body)
	}
	st = get()
	if st.Current != "kubernetes" || !st.Engines[1].Current || st.ChosenBy != "uid 0" || st.ChosenAt == "" {
		t.Errorf("after switching: %+v", st)
	}
	if name, _ := a.srv.jdp.Engines().Now(); name != "kubernetes" {
		t.Errorf("new jobs go to %s", name)
	}
	// the choice is kept: what a restart reads
	if s, err := a.srv.jobs.StoredEngine(t.Context()); err != nil || s.Value != "kubernetes" {
		t.Errorf("stored choice: %+v %v", s, err)
	}
}

// stubEngine is an engine whose status a test sets.
type stubEngine struct {
	name, err string
}

func (s *stubEngine) ExecuteJob(model.Job) error { return nil }
func (s *stubEngine) CancelJob(model.Job) error  { return nil }
func (s *stubEngine) Name() string               { return s.name }
func (s *stubEngine) Status(context.Context) model.ExecutorStatus {
	return model.ExecutorStatus{Engine: s.name, Active: s.err == "", Error: s.err}
}
