package userspace

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	ut "kyri56xcaesar/myThesis/internal/utils"

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
	j.Manager.StartWorker()
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

func (jm *JobManager) StartWorker() {
	log.Printf("starting worker")
	log.Printf("jobQueue length: %v", len(jm.jobQueue))
	go func() {
		for job := range jm.jobQueue {
			jm.workerPool <- struct{}{} // Acquire worker slot
			// the worker itself will release it
			go jm.executor.ExecuteJob(job) // Spawn worker goroutine
		}
	}()
}

func (jm *JobManager) ScheduleJob(jb ut.Job) error {
	log.Printf("scheduling job...")

	jm.mu.Lock()
	defer jm.mu.Unlock()
	// if _, exists := jm.jobs[jb.Jid]; exists {
	// 	return fmt.Errorf("job %d already exists", jb.Jid)
	// }

	jb.Status = "queued"
	jb.Created_at = time.Now().Format(time.RFC3339)
	// jm.jobs[jb.Jid] = &jb
	jm.jobQueue <- jb

	// log.Printf("Job %d submitted\n", jb.Jid)
	return nil
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

func streamToSocketWS(jobID int, pipe io.Reader) {
	jobIDStr := strconv.Itoa(jobID)
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
