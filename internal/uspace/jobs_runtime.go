package uspace

/*
	The execution runtime: what running a job needs, whichever dispatcher
	decides when and by whom it runs.

	  executors   HOW a job runs on an engine (docker, kubernetes); they
	              depend on this runtime, never on a dispatcher
	  jobRuntime  the engines and which one takes new jobs, the running jobs
	              (their cancel funcs: how a cancel reaches a running job),
	              jobs cancelled before they started, and the work in flight
	  dispatchers WHEN and BY WHOM: each owns its queue, its concurrency
	              (worker slots) and its start/drain policy
	              - JobManager    an in-memory queue served by workers
	              - DBDispatcher  the jobs table as the queue, leases, a reaper
	              - (a broker one would consume from pub/sub the same way)
*/

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// jobRuntime is shared by one dispatcher and its executors.
type jobRuntime struct {
	srv     *UService
	engines *engineSet // the executors, and the one taking new jobs

	mu        sync.Mutex
	running   map[int64]context.CancelCauseFunc // jobs being run or followed here
	cancelled map[int64]bool                    // cancelled before they started
	inflight  sync.WaitGroup                    // work handed out and not finished
}

// newJobRuntime builds the runtime with both engines; J_EXECUTOR picks the
// one for new jobs (an admin's stored choice replaces it at start).
func newJobRuntime(srv *UService) *jobRuntime {
	jobsSocketAddress = srv.config.WssAddress
	jobsServiceSecret = srv.config.ServiceSecretKey

	rt := newRuntimeWith(srv)
	rt.engines = newEngineSet(NewJDockerExecutor(rt), NewJKubernetesExecutor(rt))
	if err := rt.engines.use(engineName(srv.config.UspaceJobExecutor)); err != nil {
		panic(err)
	}

	return rt
}

// newRuntimeWith is a runtime without engines (tests add their own).
func newRuntimeWith(srv *UService, executors ...JobExecutor) *jobRuntime {
	return &jobRuntime{
		srv: srv, engines: newEngineSet(executors...),
		running: map[int64]context.CancelCauseFunc{}, cancelled: map[int64]bool{},
	}
}

// start runs work in the background, counted until it returns (for wait).
func (rt *jobRuntime) start(work func()) {
	rt.inflight.Add(1)
	go func() {
		defer rt.inflight.Done()
		work()
	}()
}

// wait waits for the work in flight, or for ctx to end.
func (rt *jobRuntime) wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() { rt.inflight.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		rt.mu.Lock()
		n := len(rt.running)
		rt.mu.Unlock()

		return fmt.Errorf("%d job(s) still running: %w", n, ctx.Err())
	}
}

// cancel stops a job: a running one has its context cancelled (its executor
// then stops it in the engine); one that hasn't started yet is remembered,
// and its executor cancels it on start (takeCancelled).
func (rt *jobRuntime) cancel(jid int64) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if cancel, ok := rt.running[jid]; ok {
		log.Printf("[jobs] cancelling running job %d", jid)
		cancel(ErrJobCancelled)

		return
	}
	rt.cancelled[jid] = true
}

// takeCancelled reports (and forgets) whether jid was cancelled before it
// started.
func (rt *jobRuntime) takeCancelled(jid int64) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.cancelled[jid] {
		delete(rt.cancelled, jid)

		return true
	}

	return false
}

// isTracked reports whether a job is being run or followed here.
func (rt *jobRuntime) isTracked(jid int64) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	_, ok := rt.running[jid]

	return ok
}

// trackRunning registers a running job's cancel func until the returned
// func is called.
func (rt *jobRuntime) trackRunning(jid int64, cancel context.CancelCauseFunc) func() {
	rt.mu.Lock()
	rt.running[jid] = cancel
	rt.mu.Unlock()

	return func() {
		rt.mu.Lock()
		delete(rt.running, jid)
		rt.mu.Unlock()
	}
}
