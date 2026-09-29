package uspace

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kyri56xcaesar/kuspace/internal/model"
	ut "kyri56xcaesar/kuspace/internal/utils"
)

// recordingExecutor records which jobs ran (the dispatcher releases the
// worker slot when ExecuteJob returns).
type recordingExecutor struct {
	mu  sync.Mutex
	ran []int64
}

func (e *recordingExecutor) ExecuteJob(job model.Job) error {
	e.mu.Lock()
	e.ran = append(e.ran, job.JID)
	e.mu.Unlock()

	return nil
}
func (e *recordingExecutor) CancelJob(model.Job) error { return nil }
func (e *recordingExecutor) Name() string              { return "test" }
func (e *recordingExecutor) Status(context.Context) model.ExecutorStatus {
	return model.ExecutorStatus{Engine: "test", Active: true}
}

func newTestManager() (*JobManager, *recordingExecutor) {
	ex := &recordingExecutor{}
	jm := &JobManager{
		rt:         newRuntimeWith(nil, ex),
		jobQueue:   make(chan model.Job, 10),
		workerPool: make(chan struct{}, 2),
		draining:   &atomic.Bool{},
	}

	return jm, ex
}

func TestCancelQueuedJobIsSkipped(t *testing.T) {
	jm, ex := newTestManager()
	if err := jm.CancelJob(5); err != nil { // cancelled before its turn
		t.Fatal(err)
	}
	for _, jid := range []int64{5, 6} {
		if err := jm.ScheduleJob(model.Job{JID: jid}); err != nil {
			t.Fatal(err)
		}
	}
	jm.StartDispatcher()
	deadline := time.After(3 * time.Second)
	for {
		ex.mu.Lock()
		n := len(ex.ran)
		ex.mu.Unlock()
		if n >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("no job ran")
		case <-time.After(10 * time.Millisecond):
		}
	}
	time.Sleep(50 * time.Millisecond)
	ex.mu.Lock()
	defer ex.mu.Unlock()
	if len(ex.ran) != 1 || ex.ran[0] != 6 {
		t.Errorf("ran %v, want only job 6 (5 was cancelled while queued)", ex.ran)
	}
	if jm.rt.takeCancelled(5) {
		t.Error("cancel mark for job 5 was not consumed")
	}
}

func TestCancelRunningJobCancelsItsContext(t *testing.T) {
	jm, _ := newTestManager()
	ctx, cancel := context.WithCancelCause(context.Background())
	untrack := jm.rt.trackRunning(7, cancel)

	if err := jm.CancelJob(7); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(ctx), ErrJobCancelled) {
		t.Fatalf("running job's context cause = %v, want ErrJobCancelled", context.Cause(ctx))
	}
	untrack()
	// once finished it's no longer "running": a later cancel only marks it
	_ = jm.CancelJob(7)
	if !jm.rt.takeCancelled(7) {
		t.Error("cancel after the job finished should fall back to the queued mark")
	}
}

func TestQueueFull(t *testing.T) {
	jm := &JobManager{rt: newRuntimeWith(nil), jobQueue: make(chan model.Job, 1), draining: &atomic.Bool{}}
	if err := jm.ScheduleJob(model.Job{JID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := jm.ScheduleJob(model.Job{JID: 2}); !errors.Is(err, ErrJobQueueFull) {
		t.Errorf("second job on a full queue: %v, want ErrJobQueueFull", err)
	}
}

// blockingExecutor runs jobs until release is closed.
type blockingExecutor struct {
	started chan int64
	release chan struct{}
}

func (b blockingExecutor) ExecuteJob(job model.Job) error {
	b.started <- job.JID
	<-b.release

	return nil
}

func (b blockingExecutor) CancelJob(model.Job) error { return nil }
func (b blockingExecutor) Name() string              { return "test" }
func (b blockingExecutor) Status(context.Context) model.ExecutorStatus {
	return model.ExecutorStatus{Engine: "test", Active: true}
}

func TestDrainWaitsForRunningJobs(t *testing.T) {
	ex := blockingExecutor{started: make(chan int64, 4), release: make(chan struct{})}
	jm := &JobManager{
		rt: newRuntimeWith(nil, ex), jobQueue: make(chan model.Job, 4), workerPool: make(chan struct{}, 2),
		draining: &atomic.Bool{},
	}
	jm.StartDispatcher()
	if err := jm.ScheduleJob(model.Job{JID: 1}); err != nil {
		t.Fatal(err)
	}
	<-ex.started

	// while a job runs, draining times out and refuses new jobs
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := jm.Drain(ctx); err == nil {
		t.Error("drain returned while a job was running")
	}
	if err := jm.ScheduleJob(model.Job{JID: 2}); !errors.Is(err, ErrShuttingDown) || ut.HTTPStatus(err) != http.StatusServiceUnavailable {
		t.Errorf("submission while draining: %v", err)
	}

	close(ex.release)
	if err := jm.Drain(context.Background()); err != nil {
		t.Errorf("drain after the job finished: %v", err)
	}
}
