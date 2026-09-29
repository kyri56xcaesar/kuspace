package jobs

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
	"time"

	"kyri56xcaesar/kuspace/internal/config"
	"kyri56xcaesar/kuspace/internal/model"
)

// Storage is what running jobs need from the file storage of the service
// they run in (uspace implements it). Jobs never get storage credentials:
// a job's container reads its input and writes its output through
// presigned URLs.
type Storage interface {
	// DefaultVolume is where an input or output named without a volume lives.
	DefaultVolume() string
	// PresignJobIO gives a job time-limited URLs to read in and write out.
	PresignJobIO(ctx context.Context, in, out model.Resource, valid time.Duration) (inputURL, outputURL string, err error)
	// RecordOutput records a completed job's output object as a file of the
	// job's owner (charging its size to their quota) and says what it did.
	RecordOutput(ctx context.Context, job model.Job) (action string, err error)
	// OutputWrittenSince reports whether the job's output object was written
	// at or after since.
	OutputWrittenSince(ctx context.Context, job model.Job, since time.Time) bool
}

// Deps are what the jobs code needs from the service it runs in.
type Deps struct {
	Config  config.EnvConfig
	Store   *Store // the jobs table (jobs.db, opened)
	Storage Storage
}

// jobRuntime is shared by one dispatcher and its executors.
type jobRuntime struct {
	config  config.EnvConfig
	store   *Store
	storage Storage
	engines *EngineSet // the executors, and the one taking new jobs

	mu        sync.Mutex
	running   map[int64]context.CancelCauseFunc // jobs being run or followed here
	cancelled map[int64]bool                    // cancelled before they started
	inflight  sync.WaitGroup                    // work handed out and not finished
}

// newJobRuntime builds the runtime with both engines; J_EXECUTOR picks the
// one for new jobs (an admin's stored choice replaces it at start).
func newJobRuntime(deps Deps) *jobRuntime {
	jobsSocketAddress = deps.Config.WssAddress
	jobsServiceSecret = deps.Config.ServiceSecretKey

	rt := newRuntimeWith(deps)
	rt.engines = newEngineSet(NewJDockerExecutor(rt), NewJKubernetesExecutor(rt))
	if err := rt.engines.Use(EngineName(deps.Config.UspaceJobExecutor)); err != nil {
		panic(err)
	}

	return rt
}

// newRuntimeWith is a runtime with the given executors (tests use their own).
func newRuntimeWith(deps Deps, executors ...JobExecutor) *jobRuntime {
	return &jobRuntime{
		config: deps.Config, store: deps.Store, storage: deps.Storage, engines: newEngineSet(executors...),
		running: map[int64]context.CancelCauseFunc{}, cancelled: map[int64]bool{},
	}
}

// what the executors record, through the runtime

func (rt *jobRuntime) markJobStatus(ctx context.Context, jid int64, status string, d time.Duration) error {
	return rt.store.MarkStatus(ctx, jid, status, d)
}

func (rt *jobRuntime) markJobEngine(ctx context.Context, jid int64, engine string) error {
	return rt.store.MarkEngine(ctx, jid, engine)
}

func (rt *jobRuntime) setJobReason(ctx context.Context, jid int64, reason string) error {
	return rt.store.SetReason(ctx, jid, reason)
}

func (rt *jobRuntime) saveJobLog(ctx context.Context, jid int64, text string) error {
	return rt.store.SaveLog(ctx, jid, text)
}

func (rt *jobRuntime) getJobByID(ctx context.Context, jid int) (model.Job, error) {
	return rt.store.ByID(ctx, jid)
}

func (rt *jobRuntime) outputWrittenSince(ctx context.Context, job model.Job, since time.Time) bool {
	return rt.storage.OutputWrittenSince(ctx, job, since)
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
