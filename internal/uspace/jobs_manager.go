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

// JobDispatcherImpl struct, just a paradeigm implementation of the JobManager interface
type JobDispatcherImpl struct {
	Manager JobManager
}

// Start method launching the Dispatcher work
func (j JobDispatcherImpl) Start() {
	j.Manager.StartDispatcher()
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

	jm := JobManager{
		mu:  &sync.Mutex{},
		srv: srv,

		// jobs:       make(map[int]*Job),
		jobQueue:   make(chan ut.Job, qs),
		workerPool: make(chan struct{}, mw),
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
			log.Printf("[Scheduler] Job received: ID=%d. Waiting for available worker slot...", job.JID)
			jm.workerPool <- struct{}{} // Acquire worker slot
			log.Printf("[Scheduler] Assigned job ID=%ds to a worker. Active workers: %d/%d",
				job.JID, len(jm.workerPool), cap(jm.workerPool))
			// the worker itself will release it
			go func() {
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

	jb.Status = "queued"
	jb.CreatedAt = ut.CurrentTime()

	select {
	case jm.jobQueue <- jb:
		log.Printf("[Scheduler] Job ID=%d added to queue. Current queue length: %d/%d",
			jb.JID, len(jm.jobQueue), cap(jm.jobQueue))

		return nil
	default:
		log.Printf("⚠️ [Scheduler] Job queue full! Job ID=%d rejected", jb.JID)

		return errors.New("job queue full")
	}
}

// CancelJob method removes a job from the execution channel queue
// Not fully functional yet
func (jm *JobManager) CancelJob(jid int) error {
	log.Printf("canceling job: %v", jid)
	jm.mu.Lock()
	defer jm.mu.Unlock()

	// if _, exists := js.jobs[jid]; !exists {
	// return fmt.Errorf("job %d not found", jid)
	// }

	// delete(js.jobs, jid)
	// log.Printf("Job %d cancelled\n", jid)
	return nil
}

func streamToSocketWS(jobID int64, ch <-chan []byte) {
	jobIDStr := strconv.FormatInt(jobID, 10)
	wsURL := fmt.Sprintf("ws://"+jobsSocketAddress+"/get-session?jid=%s&role=Producer", jobIDStr)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
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
		}
		err = resp.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}
}
