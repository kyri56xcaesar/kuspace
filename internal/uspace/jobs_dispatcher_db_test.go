package uspace

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

// world is the job engine as the tests see it: which jobs it has and in
// what state. It outlives the dispatchers, as a real engine outlives uspace.
type world struct {
	mu        sync.Mutex
	state     map[int64]engineState
	down      map[int64]bool // inspect can't answer for these
	outputs   map[int64]bool // jobs that wrote their output
	ran       map[string][]int64
	reattach  map[string][]int64
	inspected atomic.Int32
	finish    chan struct{} // closed: jobs being followed again end (succeeded)
}

func newWorld() *world {
	return &world{state: map[int64]engineState{}, down: map[int64]bool{}, outputs: map[int64]bool{},
		ran: map[string][]int64{}, reattach: map[string][]int64{}, finish: make(chan struct{})}
}

func (w *world) set(jid int64, s engineState) { w.mu.Lock(); w.state[jid] = s; w.mu.Unlock() }

func (w *world) record(m map[string][]int64, who string, jid int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	m[who] = append(m[who], jid)
}

func (w *world) list(m map[string][]int64, who string) []int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := slices.Clone(m[who])
	slices.Sort(out)

	return out
}

// fakeExecutor is one uspace process's executor over the world.
type fakeExecutor struct {
	name string
	rt   *jobRuntime
	w    *world
	hold chan struct{} // non-nil: jobs run until it is closed
	dead atomic.Bool   // the process died: nothing it does is recorded any more
}

func (e *fakeExecutor) ExecuteJob(job ut.Job) error {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	defer e.rt.trackRunning(job.JID, cancel)()
	e.w.record(e.w.ran, e.name, job.JID)
	e.w.set(job.JID, engineRunning)
	_ = e.rt.srv.markJobStatus(context.Background(), job.JID, "running", 0)
	if e.hold != nil {
		select {
		case <-e.hold:
		case <-ctx.Done():
		}
	}
	if e.dead.Load() {
		return nil
	}
	if errors.Is(context.Cause(ctx), ErrJobCancelled) {
		e.w.set(job.JID, engineGone)

		return e.rt.srv.markJobStatus(context.Background(), job.JID, "cancelled", 0)
	}
	e.w.set(job.JID, engineSucceeded)

	return e.rt.srv.markJobStatus(context.Background(), job.JID, "completed", time.Millisecond)
}

func (e *fakeExecutor) CancelJob(ut.Job) error { return nil }

func (e *fakeExecutor) Name() string { return "fake" }

func (e *fakeExecutor) Status(context.Context) ut.ExecutorStatus {
	return ut.ExecutorStatus{Engine: "fake", Active: true}
}

func (e *fakeExecutor) inspect(_ context.Context, job ut.Job) (engineState, error) {
	e.w.inspected.Add(1)
	e.w.mu.Lock()
	defer e.w.mu.Unlock()
	if e.w.down[job.JID] {
		return engineUnknown, errors.New("engine unreachable")
	}
	if s, ok := e.w.state[job.JID]; ok {
		return s, nil
	}

	return engineGone, nil
}

func (e *fakeExecutor) reattach(job ut.Job, _ time.Time) {
	e.w.record(e.w.reattach, e.name, job.JID)
	<-e.w.finish
	e.w.set(job.JID, engineSucceeded)
	_ = e.rt.srv.markJobStatus(context.Background(), job.JID, "completed", time.Millisecond)
}

// fakeClock is the dispatchers' clock: leases expire when a test says so.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.t
}

func (c *fakeClock) Advance(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

const testLease = time.Minute

// dbDispatcherHarness is a jobs database, an engine and a clock that
// several dispatchers ("uspace processes", one after another) share.
type dbDispatcherHarness struct {
	t     *testing.T
	a     *apiHarness
	w     *world
	clock *fakeClock
}

func newDBDispatcherHarness(t *testing.T) *dbDispatcherHarness {
	t.Helper()
	a := newAPIHarness(t, 1)
	a.withJobsDB()
	old := jobsSocketAddress
	jobsSocketAddress = "127.0.0.1:1" // job output streams go nowhere
	t.Cleanup(func() { jobsSocketAddress = old })

	return &dbDispatcherHarness{t: t, a: a, w: newWorld(), clock: &fakeClock{t: time.Unix(1_800_000_000, 0)}}
}

// process starts a dispatcher named name with workers slots; hold makes its
// jobs run until the returned executor's hold is closed.
func (h *dbDispatcherHarness) process(name string, workers int, hold bool) (*DBDispatcher, *fakeExecutor) {
	h.t.Helper()
	rt := newRuntimeWith(h.a.srv)
	ex := &fakeExecutor{name: name, rt: rt, w: h.w}
	if hold {
		ex.hold = make(chan struct{})
	}
	rt.engines = newEngineSet(ex)
	d, err := NewDBDispatcher(h.a.srv, rt, workers)
	if err != nil {
		h.t.Fatal(err)
	}
	d.timing = dbDispatchTiming{lease: testLease, heartbeat: 5 * time.Millisecond, poll: 10 * time.Millisecond,
		reap: 10 * time.Millisecond, maxAttempts: 2}
	d.now = h.clock.Now
	d.outputWritten = func(_ context.Context, job ut.Job, _ time.Time) bool {
		h.w.mu.Lock()
		defer h.w.mu.Unlock()

		return h.w.outputs[job.JID]
	}
	h.t.Cleanup(func() {
		ex.dead.Store(true)
		if ex.hold != nil {
			select {
			case <-ex.hold:
			default:
				close(ex.hold)
			}
		}
		d.halt()
	})

	return d, ex
}

// insert adds a job in the given state, as a previous run left it.
func (h *dbDispatcherHarness) insert(status string, attempts int, started bool) int64 {
	h.t.Helper()
	jid, err := h.a.srv.insertJob(h.t.Context(), ut.Job{UID: 1001, Input: "vol1/in", Output: "vol1/out", Logic: "bash"})
	if err != nil {
		h.t.Fatal(err)
	}
	db, _ := h.a.srv.jdbh.GetConn()
	var startedUnix any
	if started {
		startedUnix = h.clock.Now().Add(-time.Hour).Unix()
	}
	// the lease ran out an hour ago
	if _, err := db.ExecContext(h.t.Context(), `UPDATE jobs SET status = ?, attempts = ?, started_unix = ?, lease_until = ? WHERE jid = ?`,
		status, attempts, startedUnix, h.clock.Now().Add(-time.Hour).UnixMilli(), jid); err != nil {
		h.t.Fatal(err)
	}

	return jid
}

// submit inserts a job and publishes it, as the API does.
func (h *dbDispatcherHarness) submit(d *DBDispatcher) int64 {
	h.t.Helper()
	jid, err := h.a.srv.insertJob(h.t.Context(), ut.Job{UID: 1001, Input: "vol1/in", Output: "vol1/out", Logic: "bash"})
	if err != nil {
		h.t.Fatal(err)
	}
	if err := d.PublishJob(ut.Job{JID: jid}); err != nil {
		h.t.Fatal(err)
	}

	return jid
}

func (h *dbDispatcherHarness) job(jid int64) ut.Job {
	h.t.Helper()
	j, err := h.a.srv.getJobByID(h.t.Context(), int(jid))
	if err != nil {
		h.t.Fatal(err)
	}

	return j
}

func (h *dbDispatcherHarness) lease(jid int64) (int64, bool) {
	db, _ := h.a.srv.jdbh.GetConn()
	var l *int64
	_ = db.QueryRowContext(h.t.Context(), `SELECT lease_until FROM jobs WHERE jid = ?`, jid).Scan(&l)
	if l == nil {
		return 0, false
	}

	return *l, true
}

// eventually waits (up to 5s) for cond.
func (h *dbDispatcherHarness) eventually(what string, cond func() bool) {
	h.t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
		if cond() {
			return
		}
	}
	h.t.Fatalf("timed out waiting for %s", what)
}

func (h *dbDispatcherHarness) statusIs(jid int64, status string) func() bool {
	return func() bool { return h.job(jid).Status == status }
}

func TestDBDispatcherRunsJobsFromTheTable(t *testing.T) {
	h := newDBDispatcherHarness(t)
	d, _ := h.process("A", 2, false)
	d.Start()
	var jids []int64
	for range 5 {
		jids = append(jids, h.submit(d))
	}
	for _, jid := range jids {
		h.eventually("job completed", h.statusIs(jid, "completed"))
		j := h.job(jid)
		if j.Attempts != 1 || j.Engine != "fake" {
			t.Errorf("job %d: %d attempts, engine %q (recorded at claim)", jid, j.Attempts, j.Engine)
		}
		if _, leased := h.lease(jid); leased {
			t.Errorf("job %d kept its lease after finishing", jid)
		}
	}
	if got := h.w.list(h.w.ran, "A"); !slices.Equal(got, jids) {
		t.Errorf("ran %v, want each of %v once", got, jids)
	}
}

// uspace dies while a job runs and others wait: the next start follows the
// running job again (not a second run) and runs the waiting ones.
func TestDBDispatcherFollowsJobsAcrossARestart(t *testing.T) {
	h := newDBDispatcherHarness(t)
	a, exA := h.process("A", 1, true)
	a.Start()
	first, second, third := h.submit(a), h.submit(a), h.submit(a)
	h.eventually("first job running", h.statusIs(first, "running"))
	if s := h.job(second).Status; s != "queued" {
		t.Fatalf("second job: %s", s)
	}

	// A dies: its loops and heartbeats stop, the job keeps running in the engine
	exA.dead.Store(true)
	a.halt()
	h.clock.Advance(2 * testLease)

	b, _ := h.process("B", 2, false)
	b.Start()
	h.eventually("waiting jobs run by B", func() bool {
		return h.job(second).Status == "completed" && h.job(third).Status == "completed"
	})
	if got := h.w.list(h.w.reattach, "B"); !slices.Equal(got, []int64{first}) {
		t.Fatalf("B followed %v, want [%d]", got, first)
	}
	if s := h.job(first).Status; s != "running" {
		t.Errorf("the followed job: %s", s)
	}
	close(h.w.finish) // the job ends in the engine
	h.eventually("first job completed", h.statusIs(first, "completed"))

	if got := h.w.list(h.w.ran, "A"); !slices.Equal(got, []int64{first}) {
		t.Errorf("A ran %v", got)
	}
	if got := h.w.list(h.w.ran, "B"); !slices.Equal(got, []int64{second, third}) {
		t.Errorf("B ran %v, want %v (no job twice)", got, []int64{second, third})
	}
	if r := h.job(first).StatusReason; !strings.Contains(r, "followed again") {
		t.Errorf("reason: %q", r)
	}
}

// A watched job's heartbeat keeps it out of the reaper's hands however
// long it runs.
func TestDBDispatcherLeaseKeepsAWatchedJob(t *testing.T) {
	h := newDBDispatcherHarness(t)
	d, ex := h.process("A", 1, true)
	d.Start()
	jid := h.submit(d)
	h.eventually("job running", h.statusIs(jid, "running"))
	for range 6 { // three leases' worth of time, in half-lease steps
		h.clock.Advance(testLease / 2)
		time.Sleep(40 * time.Millisecond) // heartbeats and reaper rounds happen
	}
	if n := h.w.inspected.Load(); n != 0 {
		t.Errorf("the reaper asked the engine about a watched job %d time(s)", n)
	}
	if until, ok := h.lease(jid); !ok || until < h.clock.Now().UnixMilli() {
		t.Errorf("lease %d (ok %v) is behind the clock %d", until, ok, h.clock.Now().UnixMilli())
	}
	close(ex.hold)
	h.eventually("job completed", h.statusIs(jid, "completed"))
}

func TestDBDispatcherRetriesAJobThatNeverStartedOnce(t *testing.T) {
	h := newDBDispatcherHarness(t)
	neverStarted := h.insert("starting", 1, false)    // first claim lost before it started
	twice := h.insert("starting", 2, false)           // lost twice
	startedUnmarked := h.insert("starting", 1, false) // lost after the engine started it
	h.w.set(startedUnmarked, engineRunning)

	d, _ := h.process("A", 2, false)
	d.Start()
	h.eventually("retried job completed", h.statusIs(neverStarted, "completed"))
	j := h.job(neverStarted)
	if j.Attempts != 2 || !strings.Contains(j.StatusReason, "attempt 2 of 2") {
		t.Errorf("retried job: attempts %d reason %q", j.Attempts, j.StatusReason)
	}
	if j := h.job(twice); j.Status != "failed" || !strings.Contains(j.StatusReason, "claimed 2 times") {
		t.Errorf("job lost twice: %s %q", j.Status, j.StatusReason)
	}
	// the engine had it: followed, not started a second time
	h.eventually("started job followed", func() bool { return len(h.w.list(h.w.reattach, "A")) == 1 })
	if slices.Contains(h.w.list(h.w.ran, "A"), startedUnmarked) {
		t.Error("a job the engine was running was started again")
	}
	close(h.w.finish)
	h.eventually("followed job completed", h.statusIs(startedUnmarked, "completed"))
}

func TestDBDispatcherSettlesJobsThatEndedUnwatched(t *testing.T) {
	h := newDBDispatcherHarness(t)
	succeeded := h.insert("running", 1, true)
	failed := h.insert("running", 1, true)
	goneWithOutput := h.insert("running", 1, true)
	goneWithout := h.insert("running", 1, true)
	unreachable := h.insert("running", 1, true)
	h.w.set(succeeded, engineSucceeded)
	h.w.set(failed, engineFailed)
	h.w.outputs[goneWithOutput] = true
	h.w.down[unreachable] = true

	d, _ := h.process("A", 1, false)
	d.Start() // the first reaper round runs in Start
	for jid, want := range map[int64]struct{ status, reason string }{
		succeeded:      {"completed", "finished while uspace wasn't watching"},
		failed:         {"failed", "failed while uspace wasn't watching"},
		goneWithOutput: {"completed", "wrote its output"},
		goneWithout:    {"failed", "lost"},
		unreachable:    {"running", ""},
	} {
		j := h.job(jid)
		if j.Status != want.status || !strings.Contains(j.StatusReason, want.reason) {
			t.Errorf("job %d: %s %q, want %s %q", jid, j.Status, j.StatusReason, want.status, want.reason)
		}
	}
	if len(h.w.list(h.w.ran, "A")) != 0 {
		t.Error("a lost job was run again")
	}
	// the engine answers again: settled on a later round
	h.w.mu.Lock()
	delete(h.w.down, unreachable)
	h.w.state[unreachable] = engineSucceeded
	h.w.mu.Unlock()
	h.eventually("unreachable job settled", h.statusIs(unreachable, "completed"))
}

func TestDBDispatcherCancelsWaitingJobs(t *testing.T) {
	h := newDBDispatcherHarness(t)
	d, ex := h.process("A", 1, true)
	d.Start()
	busy, waiting := h.submit(d), h.submit(d)
	h.eventually("first job running", h.statusIs(busy, "running"))
	if err := d.RemoveJob(int(waiting)); err != nil {
		t.Fatal(err)
	}
	if j := h.job(waiting); j.Status != "cancelled" || j.StatusReason != "cancelled before it started" {
		t.Errorf("waiting job: %s %q", j.Status, j.StatusReason)
	}
	// a running job is stopped by its executor
	if err := d.RemoveJob(int(busy)); err != nil {
		t.Fatal(err)
	}
	h.eventually("running job cancelled", h.statusIs(busy, "cancelled"))
	close(ex.hold)
	time.Sleep(30 * time.Millisecond)
	if got := h.w.list(h.w.ran, "A"); !slices.Equal(got, []int64{busy}) {
		t.Errorf("ran %v: the cancelled job must not run", got)
	}
}

func TestDBDispatcherStartAndDrain(t *testing.T) {
	h := newDBDispatcherHarness(t)
	// accepted (inserted) but not yet queued when uspace stopped
	pending, err := h.a.srv.insertJob(t.Context(), ut.Job{UID: 1001, Input: "vol1/in", Output: "vol1/out", Logic: "bash"})
	if err != nil {
		t.Fatal(err)
	}
	d, ex := h.process("A", 1, true)
	d.Start()
	h.eventually("pending job run", h.statusIs(pending, "running"))
	if r := h.job(pending).StatusReason; !strings.Contains(r, "accepted before uspace stopped") {
		t.Errorf("reason: %q", r)
	}

	drained := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		drained <- d.Drain(ctx)
	}()
	h.eventually("draining", d.draining.Load)
	if err := d.PublishJob(ut.Job{JID: 99}); !errors.Is(err, ErrShuttingDown) {
		t.Errorf("publish while draining: %v", err)
	}
	close(ex.hold) // the running job finishes: Drain returns
	if err := <-drained; err != nil {
		t.Errorf("drain: %v", err)
	}
	if s := h.job(pending).Status; s != "completed" {
		t.Errorf("job after drain: %s", s)
	}
}
