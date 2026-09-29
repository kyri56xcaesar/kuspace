package uspace

/*
	DBDispatcher (J_DISPATCHER=database): the jobs table is the queue, so
	jobs survive a restart of uspace. The in-memory dispatcher (the default)
	stays as it was.

	A job's way through it:

	  pending   inserted by the API; PublishJob makes it
	  queued    the queue: claimed oldest first when a worker slot is free
	  starting  claimed: attempts+1, with a lease; the executor starts it
	  running   the engine runs it (the executor marks it)
	  completed / failed / cancelled

	While a job is starting or running, whoever watches it keeps its lease
	(lease_until) in the future with a heartbeat. A job past its lease has
	lost its watcher (uspace stopped or crashed); the reaper, every few
	seconds and at start, asks the engine about it:

	  running            follow it again (reattach) and finish it as usual
	  succeeded/failed   record the outcome it reached unwatched
	  gone, never ran    queue it again, once (maxAttempts); then fail it
	  gone after running completed if it wrote its output since it started,
	                     else failed ("lost")
	  engine unreachable leave it: asked again on the next round

	Every change of state is a single UPDATE conditioned on the state it
	leaves, so the claim loop, heartbeats, the reaper and the API can't
	undo each other.
*/

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

// dbDispatchTiming are the database dispatcher's clocks.
type dbDispatchTiming struct {
	lease       time.Duration // how long a watcher answers for a job without a heartbeat
	heartbeat   time.Duration // how often a watcher renews its lease
	poll        time.Duration // look for queued jobs at least this often
	reap        time.Duration // look for jobs past their lease this often
	maxAttempts int           // claims before a job that never started fails
}

var defaultDBDispatchTiming = dbDispatchTiming{
	lease: 30 * time.Second, heartbeat: 10 * time.Second,
	poll: 2 * time.Second, reap: 15 * time.Second, maxAttempts: 2,
}

// DBDispatcher is the JobDispatcher whose queue is the jobs table.
type DBDispatcher struct {
	srv    *UService
	jm     *JobManager   // the executor, worker slots, running jobs' cancel funcs, drain
	engine jobReattacher // the executor's inspect/reattach
	timing dbDispatchTiming
	now    func() time.Time
	// outputWritten tells whether a job whose container is gone wrote its
	// output (tests stand in)
	outputWritten func(ctx context.Context, job ut.Job, since time.Time) bool

	wake     chan struct{} // a job was queued
	stop     chan struct{} // closed by Drain (and halt)
	stopOnce sync.Once
	loops    sync.WaitGroup
}

// NewDBDispatcher builds the dispatcher around jm's executor, which must
// be able to inspect and reattach jobs.
func NewDBDispatcher(srv *UService, jm *JobManager) (*DBDispatcher, error) {
	engine, ok := jm.executor.(jobReattacher)
	if !ok {
		return nil, fmt.Errorf("executor %T can't reattach jobs (%w)", jm.executor, ut.ErrInvalid)
	}

	return &DBDispatcher{
		srv: srv, jm: jm, engine: engine, timing: defaultDBDispatchTiming, now: time.Now,
		outputWritten: srv.outputWrittenSince,
		wake:          make(chan struct{}, 1), stop: make(chan struct{}),
	}, nil
}

func (d *DBDispatcher) db() (*sql.DB, error) { return d.srv.jdbh.GetConn() }

func (d *DBDispatcher) ms(t time.Time) int64 { return t.UnixMilli() }

func (d *DBDispatcher) leaseEnd() int64 { return d.ms(d.now().Add(d.timing.lease)) }

// Start queues the jobs left pending by a stop, runs the reaper once (jobs
// a previous run lost), then keeps claiming and reaping. The jobs database
// must be open.
func (d *DBDispatcher) Start() {
	ctx := context.Background()
	if n, err := d.exec(ctx, `UPDATE jobs SET status = 'queued', status_reason = 'queued at start: accepted before uspace stopped'
		WHERE status = 'pending'`); err != nil {
		log.Printf("[dispatcher] queueing pending jobs: %v", err)
	} else if n > 0 {
		log.Printf("[dispatcher] queued %d job(s) accepted before the last stop", n)
	}
	d.reapOnce(ctx)
	d.loops.Add(2)
	go d.claimLoop()
	go d.reapLoop()
	d.poke()
}

// poke wakes the claim loop.
func (d *DBDispatcher) poke() {
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *DBDispatcher) exec(ctx context.Context, query string, args ...any) (int64, error) {
	db, err := d.db()
	if err != nil {
		return 0, err
	}
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

// PublishJob queues an inserted job.
func (d *DBDispatcher) PublishJob(job ut.Job) error {
	if d.jm.draining.Load() {
		return ErrShuttingDown
	}
	n, err := d.exec(context.Background(), `UPDATE jobs SET status = 'queued' WHERE jid = ? AND status = 'pending'`, job.JID)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("job %d is not waiting to be queued (%w)", job.JID, ut.ErrInvalid)
	}
	d.poke()

	return nil
}

// PublishJobs queues each job.
func (d *DBDispatcher) PublishJobs(jobs []ut.Job) error {
	for _, j := range jobs {
		if err := d.PublishJob(j); err != nil {
			return err
		}
	}

	return nil
}

// RemoveJob cancels a job: one that hasn't been claimed is cancelled in the
// table (it will never run); a starting or running one is stopped by its
// executor, which records it cancelled.
func (d *DBDispatcher) RemoveJob(jid int) error {
	n, err := d.exec(context.Background(), `UPDATE jobs SET status = 'cancelled', lease_until = NULL,
		status_reason = 'cancelled before it started' WHERE jid = ? AND status IN ('pending', 'queued')`, jid)
	if err != nil || n > 0 {
		return err
	}

	return d.jm.CancelJob(jid)
}

// RemoveJobs cancels each job.
func (d *DBDispatcher) RemoveJobs(jids []int) error {
	for _, jid := range jids {
		if err := d.RemoveJob(jid); err != nil {
			return err
		}
	}

	return nil
}

// Subscribe is not used by this dispatcher.
func (d *DBDispatcher) Subscribe(ut.Job) error { return nil }

// EngineStatus checks the executor's engine.
func (d *DBDispatcher) EngineStatus(ctx context.Context) ut.ExecutorStatus {
	return d.jm.executor.Status(ctx)
}

// Drain stops claiming and waits (until ctx ends) for the jobs this process
// watches. Jobs still running then keep running in their engine; their
// leases lapse and the next start follows them again.
func (d *DBDispatcher) Drain(ctx context.Context) error {
	d.jm.draining.Store(true)
	d.halt()

	return d.jm.Drain(ctx)
}

// halt stops the claim and reap loops and every heartbeat.
func (d *DBDispatcher) halt() {
	d.stopOnce.Do(func() { close(d.stop) })
	d.loops.Wait()
}

func (d *DBDispatcher) claimLoop() {
	defer d.loops.Done()
	tick := time.NewTicker(d.timing.poll)
	defer tick.Stop()
	for {
		select {
		case <-d.stop:
			return
		case <-d.wake:
		case <-tick.C:
		}
		d.claimAll(context.Background())
	}
}

// claimAll starts queued jobs while worker slots are free.
func (d *DBDispatcher) claimAll(ctx context.Context) {
	for !d.jm.draining.Load() {
		select {
		case d.jm.workerPool <- struct{}{}: // a slot; the executor releases it
		default:
			return
		}
		job, ok, err := d.claim(ctx)
		if err != nil || !ok {
			<-d.jm.workerPool
			if err != nil {
				log.Printf("[dispatcher] claiming a job: %v", err)
			}

			return
		}
		d.watch(ctx, job, func() {
			if err := d.jm.executor.ExecuteJob(job); err != nil {
				log.Printf("[dispatcher] job %d: %v", job.JID, err)
			}
		})
	}
}

// claim takes the oldest queued job: starting, attempts+1, leased.
func (d *DBDispatcher) claim(ctx context.Context) (ut.Job, bool, error) {
	db, err := d.db()
	if err != nil {
		return ut.Job{}, false, err
	}
	var jid int64
	err = db.QueryRowContext(ctx, `UPDATE jobs SET status = 'starting', attempts = attempts + 1, lease_until = ?
		WHERE jid = (SELECT jid FROM jobs WHERE status = 'queued' ORDER BY jid LIMIT 1) AND status = 'queued'
		RETURNING jid`, d.leaseEnd()).Scan(&jid)
	if errors.Is(err, sql.ErrNoRows) {
		return ut.Job{}, false, nil
	}
	if err != nil {
		return ut.Job{}, false, err
	}
	job, err := d.srv.getJobByID(ctx, int(jid))

	return job, err == nil, err
}

// watch runs work for job (executing or following it) under a heartbeat,
// counted for Drain; a job work leaves unfinished is failed.
func (d *DBDispatcher) watch(ctx context.Context, job ut.Job, work func()) {
	d.jm.inflight.Add(1)
	beat := d.heartbeat(ctx, job.JID)
	go func() {
		defer d.jm.inflight.Done()
		defer beat()
		work()
		if n, err := d.exec(ctx, `UPDATE jobs SET status = 'failed', lease_until = NULL,
			status_reason = 'its executor stopped without a final status' WHERE jid = ? AND status IN ('starting', 'running')`,
			job.JID); err != nil {
			log.Printf("[dispatcher] settling job %d: %v", job.JID, err)
		} else if n > 0 {
			log.Printf("[dispatcher] job %d ended without a final status: failed", job.JID)
		}
	}()
}

// heartbeat keeps jid's lease ahead until the returned func is called (or
// the dispatcher halts).
func (d *DBDispatcher) heartbeat(ctx context.Context, jid int64) func() {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		tick := time.NewTicker(d.timing.heartbeat)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-d.stop:
				return
			case <-tick.C:
				if _, err := d.exec(ctx, `UPDATE jobs SET lease_until = ?
					WHERE jid = ? AND status IN ('starting', 'running')`, d.leaseEnd(), jid); err != nil {
					log.Printf("[dispatcher] renewing the lease of job %d: %v", jid, err)
				}
			}
		}
	}()

	return func() { close(done); <-stopped }
}

func (d *DBDispatcher) reapLoop() {
	defer d.loops.Done()
	tick := time.NewTicker(d.timing.reap)
	defer tick.Stop()
	for {
		select {
		case <-d.stop:
			return
		case <-tick.C:
			d.reapOnce(context.Background())
		}
	}
}

// lostJob is a starting or running job past its lease.
type lostJob struct {
	job     ut.Job
	status  string
	started time.Time // zero when it never started
}

// reapOnce finds the jobs past their lease and settles each one.
func (d *DBDispatcher) reapOnce(ctx context.Context) {
	lost, err := d.lostJobs(ctx)
	if err != nil {
		log.Printf("[dispatcher] looking for lost jobs: %v", err)

		return
	}
	for _, l := range lost {
		if d.jm.isTracked(l.job.JID) { // watched here: its heartbeat is only late
			continue
		}
		d.settleLost(ctx, l)
	}
}

func (d *DBDispatcher) lostJobs(ctx context.Context) ([]lostJob, error) {
	db, err := d.db()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT jid, status, COALESCE(started_unix, 0) FROM jobs
		WHERE status IN ('starting', 'running') AND (lease_until IS NULL OR lease_until < ?)`, d.ms(d.now()))
	if err != nil {
		return nil, err
	}
	type row struct {
		jid     int64
		status  string
		started int64
	}
	var found []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.jid, &r.status, &r.started); err != nil {
			_ = rows.Close()

			return nil, err
		}
		found = append(found, r)
	}
	_ = rows.Close()
	lost := make([]lostJob, 0, len(found))
	for _, r := range found {
		job, err := d.srv.getJobByID(ctx, int(r.jid))
		if err != nil {
			return nil, err
		}
		l := lostJob{job: job, status: r.status}
		if r.started > 0 {
			l.started = time.Unix(r.started, 0)
		}
		lost = append(lost, l)
	}

	return lost, nil
}

// settleLost decides what becomes of one lost job (see the top of the file).
func (d *DBDispatcher) settleLost(ctx context.Context, l lostJob) {
	jid := l.job.JID
	state, err := d.engine.inspect(ctx, l.job)
	if state == engineUnknown {
		log.Printf("[dispatcher] job %d lost its watcher; the engine can't say where it is (%v): asking again later", jid, err)

		return
	}
	started := l.started
	if started.IsZero() {
		started = d.now()
	}
	switch state {
	case engineRunning:
		// take it over: running, leased, followed until it ends
		n, err := d.exec(ctx, `UPDATE jobs SET status = 'running', lease_until = ?, started_unix = COALESCE(started_unix, ?),
			status_reason = 'followed again after uspace restarted' WHERE jid = ? AND status = ? AND (lease_until IS NULL OR lease_until < ?)`,
			d.leaseEnd(), started.Unix(), jid, l.status, d.ms(d.now()))
		if err != nil || n == 0 {
			return
		}
		log.Printf("[dispatcher] job %d is still running: following it again", jid)
		d.watch(ctx, l.job, func() { d.engine.reattach(l.job, started) })
	case engineSucceeded:
		d.finishUnwatched(ctx, l, "completed", "it finished while uspace wasn't watching", started)
	case engineFailed:
		d.finishUnwatched(ctx, l, "failed", "it failed while uspace wasn't watching", started)
	case engineGone:
		switch {
		case l.status == "starting":
			d.retryOrFail(ctx, l)
		case d.outputWritten(ctx, l.job, started):
			d.finishUnwatched(ctx, l, "completed", "its container is gone, but it wrote its output", started)
		default:
			d.finishUnwatched(ctx, l, "failed", "lost: its container is gone and it wrote no output", started)
		}
	}
}

// retryOrFail queues a job that was claimed but never started again, or
// fails it once it has had its attempts.
func (d *DBDispatcher) retryOrFail(ctx context.Context, l lostJob) {
	now := d.ms(d.now())
	if l.job.Attempts < d.timing.maxAttempts {
		reason := fmt.Sprintf("queued again: it was claimed but never started (attempt %d of %d)", l.job.Attempts+1, d.timing.maxAttempts)
		if n, err := d.exec(ctx, `UPDATE jobs SET status = 'queued', lease_until = NULL, status_reason = ?
			WHERE jid = ? AND status = 'starting' AND (lease_until IS NULL OR lease_until < ?)`, reason, l.job.JID, now); err == nil && n > 0 {
			log.Printf("[dispatcher] job %d never started: queued again", l.job.JID)
			d.poke()
		}

		return
	}
	reason := fmt.Sprintf("it was claimed %d times and never started", l.job.Attempts)
	if _, err := d.exec(ctx, `UPDATE jobs SET status = 'failed', lease_until = NULL, status_reason = ?
		WHERE jid = ? AND status = 'starting' AND (lease_until IS NULL OR lease_until < ?)`, reason, l.job.JID, now); err != nil {
		log.Printf("[dispatcher] failing job %d: %v", l.job.JID, err)
	}
}

// finishUnwatched records the outcome a job reached while nobody watched it.
func (d *DBDispatcher) finishUnwatched(ctx context.Context, l lostJob, status, reason string, started time.Time) {
	// claim it first, so a second reaper round can't settle it twice
	n, err := d.exec(ctx, `UPDATE jobs SET lease_until = ? WHERE jid = ? AND status = ? AND (lease_until IS NULL OR lease_until < ?)`,
		d.leaseEnd(), l.job.JID, l.status, d.ms(d.now()))
	if err != nil || n == 0 {
		return
	}
	log.Printf("[dispatcher] job %d: %s (%s)", l.job.JID, status, reason)
	out := newJobOutput(l.job.JID)
	d.srv.finishJob(ctx, l.job, status, reason, d.now().Sub(started), out)
	if err := d.srv.saveJobLog(ctx, l.job.JID, out.Close()); err != nil {
		log.Printf("[dispatcher] saving the log of job %d: %v", l.job.JID, err)
	}
}
