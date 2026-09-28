package uspace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gorilla/websocket"
)

/*
	The Job manager implements the Job Dispatcher interface.

	wrapped around a dispatcher

	handles Job "scheduling" and "execution" logic

	essentially as its role implies... it manages jobs...
	...before...during...after execution...
	can be thought of a "master worker"
*/

// default value
var jobsSocketAddress = "localhost:8082"

// jobsServiceSecret authenticates the executor to wss as a producer
var jobsServiceSecret []byte

// ErrJobQueueFull is returned when a job can't be queued right now.
var ErrJobQueueFull = fmt.Errorf("job queue full (%w)", ut.ErrUnavailable)

// ErrShuttingDown is returned for jobs submitted while uspace drains.
var ErrShuttingDown = fmt.Errorf("uspace is shutting down (%w)", ut.ErrUnavailable)

// JobDispatcherImpl struct, just a paradeigm implementation of the JobManager interface
type JobDispatcherImpl struct {
	Manager JobManager
}

// Start method launching the Dispatcher work
func (j JobDispatcherImpl) Start() {
	j.Manager.StartDispatcher()
}

// Drain stops taking jobs and waits (until ctx ends) for the running ones.
func (j JobDispatcherImpl) Drain(ctx context.Context) error {
	return j.Manager.Drain(ctx)
}

// PublishJob method which publishes an incoming Job towards into a Queue towards execution
/* dispatching Jobs interface methods */
func (j JobDispatcherImpl) PublishJob(jb ut.Job) error {
	// log.Printf("publishing job... :%v", jb)

	return j.Manager.ScheduleJob(jb)
}

// PublishJobs method, same as PublishJob but with plurality
func (j JobDispatcherImpl) PublishJobs(jbs []ut.Job) error {
	for _, jb := range jbs {
		err := j.Manager.ScheduleJob(jb)
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoveJob method removes a Job from the Execution Queue pre or while execution
// Not fully functional
func (j JobDispatcherImpl) RemoveJob(jid int) error {
	return j.Manager.CancelJob(jid)
}

// RemoveJobs method same but with plurality
func (j JobDispatcherImpl) RemoveJobs(jids []int) error {
	for _, jid := range jids {
		err := j.Manager.CancelJob(jid)
		if err != nil {
			return err
		}
	}

	return nil
}

// Subscribe method is not yet functional
// this aims to link a JobManager to an external Queue
func (j JobDispatcherImpl) Subscribe(_ ut.Job) error {
	return nil
}

//  JobManager struct, the central definition of a JobManger
/*
a Job manager is the default implementation for a simplistic queue Job scheduling
in memory.

@alternatives:
  - a broker

@methods:
  - ScheduleJob(Job) error
  - CancelJob(Job) error
*/
type JobManager struct {
	srv *UService // reference to the Service
	mu  *sync.Mutex

	// jobs       map[int]*Job // cache of the jobs
	jobQueue   chan ut.Job   // actual queue of the jobs
	workerPool chan struct{} //

	cancelled map[int64]bool                    // queued jobs to skip (guarded by mu)
	running   map[int64]context.CancelCauseFunc // running jobs' cancel funcs (guarded by mu)

	inflight *sync.WaitGroup // jobs handed to a worker and not finished
	draining *atomic.Bool    // set by Drain: no new jobs start

	executor JobExecutor // logic defined for exetuing a Job
}

// NewJobManager function as in a constructor for JobManager struct
/* constructor for the JobManager */
func NewJobManager(srv *UService) JobManager {
	qs, err := strconv.Atoi(srv.config.UspaceJobQueueSize)
	if err != nil {
		qs = 100 // default size
	}
	mw, err := strconv.Atoi(srv.config.UspaceJobMaxWorkers)
	if err != nil {
		mw = 10 // default size
	}

	jobsSocketAddress = srv.config.WssAddress
	jobsServiceSecret = srv.config.ServiceSecretKey

	jm := JobManager{
		mu:  &sync.Mutex{},
		srv: srv,

		// jobs:       make(map[int]*Job),
		jobQueue:   make(chan ut.Job, qs),
		workerPool: make(chan struct{}, mw),
		cancelled:  map[int64]bool{},
		running:    map[int64]context.CancelCauseFunc{},
		inflight:   &sync.WaitGroup{},
		draining:   &atomic.Bool{},
	}

	executor, err := JobExecutorShipment(srv.config.UspaceJobExecutor, &jm)
	if err != nil {
		panic(err)
	}
	jm.executor = executor

	return jm
}

// StartDispatcher method launches a goroutine which handles the jobQueue channel queue
func (jm *JobManager) StartDispatcher() {
	log.Printf("[Scheduler] Starting worker")
	go func() {
		for job := range jm.jobQueue {
			if jm.draining.Load() {
				// stays "queued" in the database; picked up again at start (BACKLOG: jobs across restarts)
				log.Printf("[Scheduler] draining: job ID=%d left queued", job.JID)

				continue
			}
			if jm.takeCancelled(job.JID) {
				log.Printf("[Scheduler] Job ID=%d was cancelled while queued; skipping", job.JID)

				continue
			}
			log.Printf("[Scheduler] Job received: ID=%d. Waiting for available worker slot...", job.JID)
			jm.workerPool <- struct{}{} // Acquire worker slot
			log.Printf("[Scheduler] Assigned job ID=%ds to a worker. Active workers: %d/%d",
				job.JID, len(jm.workerPool), cap(jm.workerPool))
			// the worker itself will release it
			jm.inflight.Add(1)
			go func() {
				defer jm.inflight.Done()
				err := jm.executor.ExecuteJob(job) // spawn worker goroutine
				if err != nil {
					log.Printf("execution of job: %v failed.", job.JID)
				}
			}()
		}
	}()
}

// ScheduleJob method puts a job into the execution queue
func (jm *JobManager) ScheduleJob(jb ut.Job) error {
	log.Printf("[Scheduler] Scheduling job... ID=%d", jb.JID)
	if jm.draining.Load() {
		return ErrShuttingDown
	}

	jb.Status = "queued"
	jb.CreatedAt = ut.CurrentTime()

	select {
	case jm.jobQueue <- jb:
		log.Printf("[Scheduler] Job ID=%d added to queue. Current queue length: %d/%d",
			jb.JID, len(jm.jobQueue), cap(jm.jobQueue))

		return nil
	default:
		log.Printf("⚠️ [Scheduler] Job queue full! Job ID=%d rejected", jb.JID)

		return ErrJobQueueFull
	}
}

// Drain stops starting jobs (new submissions get ErrShuttingDown, queued
// ones stay queued in the database) and waits for the running ones to
// finish, or for ctx to end. Jobs still running then keep running in their
// containers; their status is settled when uspace starts again.
func (jm *JobManager) Drain(ctx context.Context) error {
	jm.draining.Store(true)
	done := make(chan struct{})
	go func() { jm.inflight.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		jm.mu.Lock()
		n := len(jm.running)
		jm.mu.Unlock()

		return fmt.Errorf("%d job(s) still running: %w", n, ctx.Err())
	}
}

// ErrJobCancelled is the cause a running job's context is cancelled with
// when a user cancels it (as opposed to hitting its deadline).
var ErrJobCancelled = errors.New("cancelled by user")

// CancelJob stops a job: a running one has its context cancelled (the
// executor then deletes it from the cluster); a queued one is skipped when
// its turn comes.
func (jm *JobManager) CancelJob(jid int) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if cancel, ok := jm.running[int64(jid)]; ok {
		log.Printf("[Scheduler] cancelling running job %d", jid)
		cancel(ErrJobCancelled)

		return nil
	}
	log.Printf("[Scheduler] job %d will be skipped when dequeued", jid)
	jm.cancelled[int64(jid)] = true

	return nil
}

// takeCancelled reports (and forgets) whether jid was cancelled while queued.
func (jm *JobManager) takeCancelled(jid int64) bool {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if jm.cancelled[jid] {
		delete(jm.cancelled, jid)

		return true
	}

	return false
}

// trackRunning registers a running job's cancel func until the returned
// func is called.
func (jm *JobManager) trackRunning(jid int64, cancel context.CancelCauseFunc) func() {
	jm.mu.Lock()
	jm.running[jid] = cancel
	jm.mu.Unlock()

	return func() {
		jm.mu.Lock()
		delete(jm.running, jid)
		jm.mu.Unlock()
	}
}

func streamToSocketWS(jobID int64, ch <-chan []byte) {
	jobIDStr := strconv.FormatInt(jobID, 10)
	wsURL := fmt.Sprintf("ws://"+jobsSocketAddress+"/get-session?jid=%s&role=Producer", jobIDStr)

	// wss only accepts producers that authenticate as a service
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"X-Service-Secret": {string(jobsServiceSecret)}})
	if err != nil {
		log.Printf("failed to connect to WS server: %v", err)

		return
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close the response body: %v", err)
		}
		err = conn.Close()
		if err != nil {
			log.Printf("failed to close the connection: %v", err)
		}
	}()

	for msg := range ch {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("failed to write to the websocket writer: %v", err)

			return
		}
	}
}

func streamToSocket(jobID int, pipe io.Reader) {
	jobIDStr := strconv.Itoa(jobID)
	scanner := bufio.NewScanner(pipe)

	log.Printf("streamToSocket function called")

	for scanner.Scan() {
		line := scanner.Text()

		// log.Printf("line about to be streamed: %s", line)

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
			fmt.Sprintf("http://"+jobsSocketAddress+"/get-session?jid=%s&role=Producer", jobIDStr),
			strings.NewReader(line),
		)
		if err != nil {
			log.Printf("failed to send log line to socket server: %v", err)

			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("failed to perform the request: %v", err)

			continue
		}
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}
}
