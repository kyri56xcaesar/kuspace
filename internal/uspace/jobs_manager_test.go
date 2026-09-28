package uspace

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

// recordingExecutor records which jobs ran and releases the worker slot as
// the real executors do.
type recordingExecutor struct {
	jm  *JobManager
	mu  sync.Mutex
	ran []int64
}

func (e *recordingExecutor) ExecuteJob(job ut.Job) error {
	defer func() { <-e.jm.workerPool }()
	e.mu.Lock()
	e.ran = append(e.ran, job.JID)
	e.mu.Unlock()

	return nil
}
func (e *recordingExecutor) CancelJob(ut.Job) error { return nil }

func newTestManager() (*JobManager, *recordingExecutor) {
	jm := &JobManager{
		mu:         &sync.Mutex{},
		jobQueue:   make(chan ut.Job, 10),
		workerPool: make(chan struct{}, 2),
		cancelled:  map[int64]bool{},
		running:    map[int64]context.CancelCauseFunc{},
		inflight:   &sync.WaitGroup{},
		draining:   &atomic.Bool{},
	}
	ex := &recordingExecutor{jm: jm}
	jm.executor = ex

	return jm, ex
}

func TestCancelQueuedJobIsSkipped(t *testing.T) {
	jm, ex := newTestManager()
	if err := jm.CancelJob(5); err != nil { // cancelled before its turn
		t.Fatal(err)
	}
	for _, jid := range []int64{5, 6} {
		if err := jm.ScheduleJob(ut.Job{JID: jid}); err != nil {
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
	if jm.takeCancelled(5) {
		t.Error("cancel mark for job 5 was not consumed")
	}
}

func TestCancelRunningJobCancelsItsContext(t *testing.T) {
	jm, _ := newTestManager()
	ctx, cancel := context.WithCancelCause(context.Background())
	untrack := jm.trackRunning(7, cancel)

	if err := jm.CancelJob(7); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(ctx), ErrJobCancelled) {
		t.Fatalf("running job's context cause = %v, want ErrJobCancelled", context.Cause(ctx))
	}
	untrack()
	// once finished it's no longer "running": a later cancel only marks it
	_ = jm.CancelJob(7)
	if !jm.takeCancelled(7) {
		t.Error("cancel after the job finished should fall back to the queued mark")
	}
}

func TestQueueFull(t *testing.T) {
	jm := &JobManager{mu: &sync.Mutex{}, jobQueue: make(chan ut.Job, 1), draining: &atomic.Bool{}}
	if err := jm.ScheduleJob(ut.Job{JID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := jm.ScheduleJob(ut.Job{JID: 2}); !errors.Is(err, ErrJobQueueFull) {
		t.Errorf("second job on a full queue: %v, want ErrJobQueueFull", err)
	}
}

// blockingExecutor runs jobs until release is closed.
type blockingExecutor struct {
	jm      *JobManager
	started chan int64
	release chan struct{}
}

func (b blockingExecutor) ExecuteJob(job ut.Job) error {
	defer func() { <-b.jm.workerPool }()
	b.started <- job.JID
	<-b.release

	return nil
}

func (b blockingExecutor) CancelJob(ut.Job) error { return nil }

func TestDrainWaitsForRunningJobs(t *testing.T) {
	jm := &JobManager{
		mu: &sync.Mutex{}, jobQueue: make(chan ut.Job, 4), workerPool: make(chan struct{}, 2),
		cancelled: map[int64]bool{}, running: map[int64]context.CancelCauseFunc{},
		inflight: &sync.WaitGroup{}, draining: &atomic.Bool{},
	}
	ex := blockingExecutor{jm: jm, started: make(chan int64, 4), release: make(chan struct{})}
	jm.executor = ex
	jm.StartDispatcher()
	if err := jm.ScheduleJob(ut.Job{JID: 1}); err != nil {
		t.Fatal(err)
	}
	<-ex.started

	// while a job runs, draining times out and refuses new jobs
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := jm.Drain(ctx); err == nil {
		t.Error("drain returned while a job was running")
	}
	if err := jm.ScheduleJob(ut.Job{JID: 2}); !errors.Is(err, ErrShuttingDown) || ut.HTTPStatus(err) != http.StatusServiceUnavailable {
		t.Errorf("submission while draining: %v", err)
	}

	close(ex.release)
	if err := jm.Drain(context.Background()); err != nil {
		t.Errorf("drain after the job finished: %v", err)
	}
}
