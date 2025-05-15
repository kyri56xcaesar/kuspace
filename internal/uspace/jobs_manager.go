package uspace

import (
	"bufio"
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

// default value
var jobs_socket_address string = "localhost:8082"

/*
a Wrapper struct containing a Job Manager.

	@look at JobManager for more details

	the wrapper exists to implement the JobDispatcher interface
	and to provide an option for future enhancements for scheduling jobs
	or connecting to a broker
*/
type JobDispatcherImpl struct {
	Manager JobManager
}

func (j JobDispatcherImpl) Start() {
	j.Manager.StartDispatcher()
}

/* dispatching Jobs interface methods */
func (j JobDispatcherImpl) PublishJob(jb ut.Job) error {
	// log.Printf("publishing job... :%v", jb)
	return j.Manager.ScheduleJob(jb)
}

func (j JobDispatcherImpl) PublishJobs(jbs []ut.Job) error {
	for _, jb := range jbs {
		err := j.Manager.ScheduleJob(jb)
		if err != nil {
			return err
		}
	}
	return nil
}

func (j JobDispatcherImpl) RemoveJob(jid int) error {
	return j.Manager.CancelJob(jid)
}

func (j JobDispatcherImpl) RemoveJobs(jids []int) error {
	for _, jid := range jids {
		err := j.Manager.CancelJob(jid)
		if err != nil {
			return err
		}
	}
	return nil
}

func (j JobDispatcherImpl) Subscribe(job ut.Job) error {
	return nil
}

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

/* constructor for the JobManager */
func NewJobManager(srv *UService) JobManager {
	qs, err := strconv.Atoi(srv.config.J_QUEUE_SIZE)
	if err != nil {
		qs = 100 // default size
	}
	mw, err := strconv.Atoi(srv.config.J_MAX_WORKERS)
	if err != nil {
		mw = 10 // default size
	}

	jm := JobManager{
		mu:  &sync.Mutex{},
		srv: srv,

		// jobs:       make(map[int]*Job),
		jobQueue:   make(chan ut.Job, qs),
		workerPool: make(chan struct{}, mw),
	}

	executor, err := JobExecutorShipment(srv.config.J_EXECUTOR, &jm)
	if err != nil {
		panic(err)
	}
	jm.executor = executor

	return jm
}

func (jm *JobManager) StartDispatcher() {
	log.Printf("[Scheduler] Starting worker")
	go func() {
		for job := range jm.jobQueue {
			log.Printf("[Scheduler] Job received: ID=%d. Waiting for available worker slot...", job.Jid)
			jm.workerPool <- struct{}{} // Acquire worker slot
			log.Printf("[Scheduler] Assigned job ID=%ds to a worker. Active workers: %d/%d", job.Jid, len(jm.workerPool), cap(jm.workerPool))
			// the worker itself will release it
			go jm.executor.ExecuteJob(job) // Spawn worker goroutine
		}
	}()
}

func (jm *JobManager) ScheduleJob(jb ut.Job) error {
	log.Printf("[Scheduler] Scheduling job... ID=%d", jb.Jid)

	jb.Status = "queued"
	jb.Created_at = ut.CurrentTime()

	select {
	case jm.jobQueue <- jb:
		log.Printf("[Scheduler] Job ID=%d added to queue. Current queue length: %d/%d", jb.Jid, len(jm.jobQueue), cap(jm.jobQueue))
		return nil
	default:
		log.Printf("⚠️ [Scheduler] Job queue full! Job ID=%d rejected", jb.Jid)
		return fmt.Errorf("job queue full")
	}
}

func (js *JobManager) CancelJob(jid int) error {
	log.Printf("canceling job: %v", jid)
	js.mu.Lock()
	defer js.mu.Unlock()

	// if _, exists := js.jobs[jid]; !exists {
	// 	return fmt.Errorf("job %d not found", jid)
	// }

	// delete(js.jobs, jid)
	// log.Printf("Job %d cancelled\n", jid)
	return nil
}

func streamToSocketWS(jobID int64, pipe io.Reader) {
	jobIDStr := fmt.Sprintf("%d", jobID)
	wsURL := fmt.Sprintf("ws://"+jobs_socket_address+"/job-stream?jid=%s&role=Producer", jobIDStr)
	log.Printf("ws_url: %s", wsURL)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Printf("failed to connect to WS server: %v", err)
		return
	}
	defer conn.Close()

	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
			log.Printf("write failed: %v", err)
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

		_, err := http.Post(
			fmt.Sprintf("http://"+jobs_socket_address+"/job-stream?jid=%s&role=Producer", jobIDStr),
			"text/plain",
			strings.NewReader(line),
		)
		if err != nil {
			log.Printf("failed to send log line to socket server: %v", err)
		}
	}
}
