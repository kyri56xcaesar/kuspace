package userspace

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	ut "kyri56xcaesar/myThesis/internal/utils"

	"github.com/gorilla/websocket"
)

var jobs_socket_address string = "localhost:8082"

type JDispatcher struct {
	Manager JobManager
}

func (j JDispatcher) Start() {
	j.Manager.StartWorker()
}

/* dispatching Jobs interface methods */
func (j JDispatcher) PublishJob(jb ut.Job) error {
	// log.Printf("publishing job... :%v", jb)
	return j.Manager.ScheduleJob(jb)
}

func (j JDispatcher) PublishJobs(jbs []ut.Job) error {
	for _, jb := range jbs {
		err := j.Manager.ScheduleJob(jb)
		if err != nil {
			return err
		}
	}
	return nil
}

func (j JDispatcher) RemoveJob(jid int) error {
	return j.Manager.CancelJob(jid)
}

func (j JDispatcher) RemoveJobs(jids []int) error {
	for _, jid := range jids {
		err := j.Manager.CancelJob(jid)
		if err != nil {
			return err
		}
	}
	return nil
}

func (j JDispatcher) Subscribe(job ut.Job) error {
	return nil
}

/*
a Job manager is the default implementation for publishing/dispatching Jobs
as well as scheduling and cancelling Jobs. It is a simple in-memory implementation

@alternatives:
  - a broker

@methods:
  - ScheduleJob(Job) error
  - CancelJob(Job) error
*/
type JobManager struct {
	mu *sync.Mutex
	// jobs       map[int]*Job
	jobQueue   chan ut.Job
	workerPool chan struct{}
	srv        *UService
}

func NewJobManager(queueSize, maxWorkers int, srv *UService) JobManager {
	return JobManager{
		mu: &sync.Mutex{},
		// jobs:       make(map[int]*Job),
		jobQueue:   make(chan ut.Job, queueSize),
		workerPool: make(chan struct{}, maxWorkers),
		srv:        srv,
	}
}

func (jm *JobManager) StartWorker() {
	log.Printf("starting worker")
	log.Printf("jobQueue length: %v", len(jm.jobQueue))
	go func() {
		for job := range jm.jobQueue {
			jm.workerPool <- struct{}{} // Acquire worker slot
			go jm.executeJob(job)       // Spawn worker goroutine
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

func (jm *JobManager) executeJob(job ut.Job) {
	// log.Printf("executing job: %+v", job)
	defer func() { <-jm.workerPool }() // Release worker slot

	jm.mu.Lock()
	job.Status = "running"
	jm.mu.Unlock()

	// we should examine input "resources"

	// language and version
	default_v_path = jm.srv.config.VOLUMES_PATH
	cmd, duration, err := prepareExecution(job, true)
	if err != nil {
		log.Printf("failed to prepare or perform job: %v", err)
		return
	}
	job.Duration = duration.Abs().Seconds()

	// output should be streamed back ...
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("error creating stdout pipe: %v", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("error creating stderr pipe: %v", err)
		return
	}

	// Start the command
	log.Printf("starting job execution")
	if err := cmd.Start(); err != nil {
		log.Printf("error starting command: %v", err)
		jm.updateJobStatus(job.Jid, "failed", 0)
		return
	}
	log.Printf("streaming to socket")
	go streamToSocketWS(job.Jid, stdout)
	go streamToSocketWS(job.Jid, stderr)

	log.Printf("waiting...")
	err = cmd.Wait()
	if err != nil {
		log.Printf("Job %d failed: %s\n", job.Jid, err)
		jm.updateJobStatus(job.Jid, "failed", 0)
		return
	}

	log.Printf("Job %d completed successfully\n", job.Jid)
	jm.updateJobStatus(job.Jid, "completed", duration)

	// insert the output resource
	go jm.syncOutputResource(job)

	// should cleanup the tmps, etc..

}

func (jm *JobManager) updateJobStatus(jid int, status string, duration time.Duration) {
	log.Printf("updating %v job status: %v", jid, status)
	err := jm.srv.MarkJobStatus(jid, status, duration)
	if err != nil {
		log.Printf("failed to update job %d status (%s): %v", jid, status, err)
	}

}

func (jm *JobManager) syncOutputResource(job ut.Job) {
	fInfo, err := os.Stat(jm.srv.config.VOLUMES_PATH + "/output/" + job.Output)
	if err != nil {
		log.Printf("failed to find/stat the output file: %v", err)
		return
	}

	current_time := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
	resource := ut.Resource{
		Name:        "/output/" + job.Output,
		Type:        "file",
		Created_at:  current_time,
		Updated_at:  current_time,
		Accessed_at: current_time,
		Perms:       "rw-r--r--",
		Rid:         0,
		Uid:         job.Uid,
		Vid:         0,
		Gid:         job.Uid,
		Size:        fInfo.Size(),
		Links:       0,
	}

	err = jm.srv.storage.Insert(resource)
	if err != nil {
		log.Printf("failed to insert the resource")
	}
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

		log.Printf("line about to be streamed: %s", line)

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
