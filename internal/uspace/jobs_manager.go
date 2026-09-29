package uspace

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gorilla/websocket"
	"kyri56xcaesar/kuspace/internal/model"
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

func (j JobDispatcherImpl) engines() *engineSet { return j.Manager.rt.engines }

// PublishJob method which publishes an incoming Job towards into a Queue towards execution
/* dispatching Jobs interface methods */
func (j JobDispatcherImpl) PublishJob(jb model.Job) error {
	// log.Printf("publishing job... :%v", jb)

	return j.Manager.ScheduleJob(jb)
}

// PublishJobs method, same as PublishJob but with plurality
func (j JobDispatcherImpl) PublishJobs(jbs []model.Job) error {
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
func (j JobDispatcherImpl) Subscribe(_ model.Job) error {
	return nil
}

//  JobManager struct, the central definition of a JobManger
/*
a Job manager is the default implementation for a simplistic queue Job scheduling
in memory: a channel of jobs served by a pool of workers. Running a job (the
engines, cancellation, the work in flight) is the runtime's (jobs_runtime.go),
shared with the other dispatchers.

@alternatives:
  - the jobs table (DBDispatcher)
  - a broker

@methods:
  - ScheduleJob(Job) error
  - CancelJob(Job) error
*/
type JobManager struct {
	rt *jobRuntime // the engines, running jobs, work in flight

	jobQueue   chan model.Job // actual queue of the jobs
	workerPool chan struct{}  // a slot per running job (J_MAX_WORKERS)

	draining *atomic.Bool // set by Drain: no new jobs start
}

// NewJobManager function as in a constructor for JobManager struct
/* constructor for the JobManager */
func NewJobManager(srv *UService) JobManager {
	return newJobManagerOn(newJobRuntime(srv), srv)
}

// newJobManagerOn builds the in-memory dispatcher on a runtime.
func newJobManagerOn(rt *jobRuntime, srv *UService) JobManager {
	qs, err := strconv.Atoi(srv.config.UspaceJobQueueSize)
	if err != nil {
		qs = 100 // default size
	}

	return JobManager{
		rt:         rt,
		jobQueue:   make(chan model.Job, qs),
		workerPool: make(chan struct{}, maxWorkers(srv)),
		draining:   &atomic.Bool{},
	}
}

// maxWorkers is J_MAX_WORKERS: how many jobs a dispatcher runs at once.
func maxWorkers(srv *UService) int {
	mw, err := strconv.Atoi(srv.config.UspaceJobMaxWorkers)
	if err != nil || mw <= 0 {
		return 10 // default size
	}

	return mw
}

// StartDispatcher method launches a goroutine which handles the jobQueue channel queue
func (jm *JobManager) StartDispatcher() {
	log.Printf("[Scheduler] Starting worker")
	go func() {
		for job := range jm.jobQueue {
			if jm.draining.Load() {
				// stays "queued" in the database (the database dispatcher picks such jobs up at start)
				log.Printf("[Scheduler] draining: job ID=%d left queued", job.JID)

				continue
			}
			if jm.rt.takeCancelled(job.JID) {
				log.Printf("[Scheduler] Job ID=%d was cancelled while queued; skipping", job.JID)

				continue
			}
			log.Printf("[Scheduler] Job received: ID=%d. Waiting for available worker slot...", job.JID)
			jm.workerPool <- struct{}{} // Acquire worker slot
			log.Printf("[Scheduler] Assigned job ID=%ds to a worker. Active workers: %d/%d",
				job.JID, len(jm.workerPool), cap(jm.workerPool))
			jm.rt.start(func() {
				defer func() { <-jm.workerPool }() // release the worker slot
				name, executor := jm.rt.engines.now()
				job.Engine = name
				if err := executor.ExecuteJob(job); err != nil {
					log.Printf("execution of job: %v failed.", job.JID)
				}
			})
		}
	}()
}

// ScheduleJob method puts a job into the execution queue
func (jm *JobManager) ScheduleJob(jb model.Job) error {
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
// containers.
func (jm *JobManager) Drain(ctx context.Context) error {
	jm.draining.Store(true)

	return jm.rt.wait(ctx)
}

// ErrJobCancelled is the cause a running job's context is cancelled with
// when a user cancels it (as opposed to hitting its deadline).
var ErrJobCancelled = errors.New("cancelled by user")

// CancelJob stops a job: a running one has its context cancelled (the
// executor then deletes it from the cluster); a queued one is skipped when
// its turn comes.
func (jm *JobManager) CancelJob(jid int) error {
	jm.rt.cancel(int64(jid))

	return nil
}

func streamToSocketWS(address string, jobID int64, ch <-chan []byte) {
	jobIDStr := strconv.FormatInt(jobID, 10)
	wsURL := fmt.Sprintf("ws://"+address+"/get-session?jid=%s&role=Producer", jobIDStr)

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
