package uspace

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestParseJobQuotas(t *testing.T) {
	good := ut.Job{MemoryRequest: "512Mi", MemoryLimit: "1Gi", CPURequest: "500m", CPULimit: "1"}
	q, err := parseJobQuotas(good)
	if err != nil {
		t.Fatalf("valid quotas rejected: %v", err)
	}
	if q.limMem.String() != "1Gi" || q.reqCPU.String() != "500m" {
		t.Errorf("parsed quotas = %+v", q)
	}

	bad := good
	bad.CPULimit = "abc" // resource.MustParse used to panic here and kill uspace
	if _, err := parseJobQuotas(bad); err == nil {
		t.Error("invalid cpu limit accepted")
	}
	if err := validateJobQuotas(bad); err == nil {
		t.Error("submission check accepted an invalid cpu limit")
	}
	if err := validateJobQuotas(ut.Job{}); err != nil {
		t.Errorf("empty quotas (defaults apply later) rejected: %v", err)
	}
}

func TestBuildK8sJobUsesOneName(t *testing.T) {
	q, _ := parseJobQuotas(ut.Job{MemoryRequest: "1Gi", MemoryLimit: "2Gi", CPURequest: "1", CPULimit: "2"})
	j := buildK8sJob(k8sJobName(42), "img", []string{"run"}, map[string]string{"A": "b"}, q, 1, "ns", 60, 30)

	// create, watch, cancel and pod lookup all derive from k8sJobName
	if j.Name != "job-42" || k8sJobName(42) != j.Name {
		t.Errorf("job name = %q, want job-42", j.Name)
	}
	c := j.Spec.Template.Spec.Containers[0]
	if c.Resources.Limits.Memory().String() != "2Gi" || c.Resources.Requests.Cpu().String() != "1" {
		t.Errorf("resources = %+v", c.Resources)
	}
	if *j.Spec.ActiveDeadlineSeconds != 60 || *j.Spec.BackoffLimit != 0 {
		t.Errorf("deadline/backoff = %d/%d", *j.Spec.ActiveDeadlineSeconds, *j.Spec.BackoffLimit)
	}
}

// fakeWatches makes each Watch call return the next scripted watcher.
func fakeWatches(t *testing.T, scripts ...func(w *watch.FakeWatcher)) *fake.Clientset {
	t.Helper()
	cs := fake.NewSimpleClientset()
	calls := 0
	cs.PrependWatchReactor("jobs", func(k8stesting.Action) (bool, watch.Interface, error) {
		w := watch.NewFakeWithChanSize(10, false)
		if calls < len(scripts) {
			scripts[calls](w)
		}
		calls++

		return true, w, nil
	})

	return cs
}

func jobWith(status batchv1.JobStatus) *batchv1.Job {
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "job-1"}, Status: status}
}

func TestMonitorJobSurvivesErrorEventsAndReconnects(t *testing.T) {
	cs := fakeWatches(t,
		// first watch: an error event (not a *Job - used to panic), then it closes
		func(w *watch.FakeWatcher) {
			w.Error(&metav1.Status{Message: "too old resource version"})
			w.Stop()
		},
		// second watch: the job is still running, then succeeds
		func(w *watch.FakeWatcher) {
			w.Modify(jobWith(batchv1.JobStatus{Active: 1}))
			w.Modify(jobWith(batchv1.JobStatus{Succeeded: 1}))
		},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := monitorJob(ctx, cs, "job-1", "ns")
	if err != nil || status != "completed" {
		t.Fatalf("monitorJob = %q, %v; want completed", status, err)
	}
}

func TestMonitorJobFailedAndTimeout(t *testing.T) {
	cs := fakeWatches(t, func(w *watch.FakeWatcher) { w.Modify(jobWith(batchv1.JobStatus{Failed: 1})) })
	status, err := monitorJob(context.Background(), cs, "job-1", "ns")
	if err != nil || status != "failed" {
		t.Fatalf("monitorJob = %q, %v; want failed", status, err)
	}

	// a job that never finishes returns when the context ends
	cs = fakeWatches(t) // watches that never report anything
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if status, err := monitorJob(ctx, cs, "job-1", "ns"); err == nil || status != "unknown" {
			t.Errorf("monitorJob on a stuck job = %q, %v", status, err)
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("monitorJob ignored its context")
	}
}

func TestJobOutput(t *testing.T) {
	o := &jobOutput{ch: make(chan []byte, 1)} // no websocket in tests
	o.Send([]byte("first"))
	o.Send([]byte("second\n")) // channel full: dropped live, still kept
	kept := o.Close()
	if !strings.Contains(kept, "first\n") || !strings.Contains(kept, "second\n") {
		t.Errorf("kept log = %q", kept)
	}
	o.Send([]byte("late")) // after Close: must not panic (send on closed channel)
	if o.Close() != kept {
		t.Error("send after close changed the log")
	}

	big := &jobOutput{ch: make(chan []byte, 1)}
	chunk := strings.Repeat("x", 1024) + "\n"
	for range 100 {
		big.Send([]byte(chunk))
	}
	big.Send([]byte("the end\n"))
	if got := big.Close(); len(got) > jobLogLimit || !strings.HasSuffix(got, "the end\n") {
		t.Errorf("kept %d bytes (limit %d), tail kept: %v", len(got), jobLogLimit, strings.HasSuffix(got, "the end\n"))
	}
}

type fakePresigner struct{ calls []string }

func (f *fakePresigner) PresignFor(_ context.Context, method string, r ut.Resource, d time.Duration) (*url.URL, error) {
	f.calls = append(f.calls, method+" "+r.Vname+"/"+r.Name+" "+d.String())

	return url.Parse("http://minio:9000/" + r.Vname + "/" + r.Name + "?X-Amz-Signature=sig")
}

func TestPresignJobIO(t *testing.T) {
	p := &fakePresigner{}
	in := ut.Resource{Vname: "vol", Name: "in.csv"}
	out := ut.Resource{Vname: "vol", Name: "out.csv"}
	get, put, err := presignJobIO(p, in, out, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(get, "/vol/in.csv") || !strings.Contains(put, "/vol/out.csv") {
		t.Errorf("urls = %s | %s", get, put)
	}
	if len(p.calls) != 2 || p.calls[0] != "get vol/in.csv 1h0m0s" || p.calls[1] != "put vol/out.csv 1h0m0s" {
		t.Errorf("presign calls = %v (want a GET for the input and a PUT for the output only)", p.calls)
	}
	// a backend that can't presign is an error, never a fallback to credentials
	if _, _, err := presignJobIO(struct{}{}, in, out, time.Hour); err == nil {
		t.Error("non-presigning storage accepted")
	}
}

func TestJobURLValidity(t *testing.T) {
	if got := jobURLValidity(30); got != 45*time.Minute {
		t.Errorf("30 min job: %v", got)
	}
	if got := jobURLValidity(0); got != 6*time.Hour {
		t.Errorf("no timeout: %v", got)
	}
}

func TestFormatJobCommandCodeModes(t *testing.T) {
	code := `print('it''s "quoted" $HOME')`
	job := ut.Job{Logic: "py:3.12", LogicBody: code}
	cmd, err := formatJobCommand(&job)
	if err != nil {
		t.Fatal(err)
	}
	if job.Logic != "python:3.12" {
		t.Errorf("image = %q, want python:3.12 (alias + version)", job.Logic)
	}
	if strings.Contains(strings.Join(cmd, " "), "quoted") {
		t.Errorf("code pasted into the command (quoting breaks): %v", cmd)
	}
	if cmd[len(cmd)-1] != `python3 -c "$LOGIC"` {
		t.Errorf("command = %v", cmd)
	}

	// without a tag a language gets its small pinned image (":latest" pulled ~1 GB for python)
	for lang, image := range map[string]string{"js": "node:22-alpine", "c": "gcc:14", "java": "eclipse-temurin:21-jdk-alpine", "golang": "golang:1.24-alpine", "python": "python:3.12-alpine"} {
		j := ut.Job{Logic: lang}
		if _, err := formatJobCommand(&j); err != nil || j.Logic != image {
			t.Errorf("%s -> %q, %v; want %s", lang, j.Logic, err, image)
		}
	}
	for _, bad := range []string{"rust", "solidity", "visual-basic"} {
		j := ut.Job{Logic: bad}
		if _, err := formatJobCommand(&j); err == nil || !strings.Contains(err.Error(), "python") {
			t.Errorf("%s: %v (want an error listing the supported languages)", bad, err)
		}
	}
	// built-in applications are untouched
	j := ut.Job{Logic: "duckdb"}
	if cmd, err := formatJobCommand(&j); err != nil || j.Logic != duckImage || cmd[1] != "duckdb_app.py" {
		t.Errorf("duckdb app: %v %v %v", j.Logic, cmd, err)
	}
}

// web/tests/code_modes.json mirrors codeModes for the editor starters'
// tests (web/tests/starters.test.js run each starter with its language's
// command). This keeps the two from drifting; UPDATE_GOLDEN=1 rewrites it.
func TestCodeModesGolden(t *testing.T) {
	type mode struct {
		Image string `json:"image"`
		Tag   string `json:"tag"`
		Run   string `json:"run"`
	}
	modes := map[string]mode{}
	for name, m := range codeModes {
		modes[name] = mode{m.image, m.tag, m.run}
	}
	want, err := json.MarshalIndent(modes, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	path := filepath.Join("..", "..", "web", "tests", "code_modes.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(want) {
		t.Errorf("%s is out of date with codeModes (UPDATE_GOLDEN=1 go test ./internal/uspace -run CodeModesGolden): %v", path, err)
	}
}
