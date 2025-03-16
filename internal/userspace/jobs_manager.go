package userspace

import (
	"log"
	"os"
	"sync"
	"time"
)

type JDispatcher struct {
	Manager JobManager
}

func (j JDispatcher) Start() {
	j.Manager.StartWorker()
}

/* dispatching Jobs interface methods */
func (j JDispatcher) PublishJob(jb Job) error {
	// log.Printf("publishing job... :%v", jb)
	return j.Manager.ScheduleJob(jb)
}

func (j JDispatcher) PublishJobs(jbs []Job) error {
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

func (j JDispatcher) Subscribe(job Job) error {
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
	jobQueue   chan Job
	workerPool chan struct{}
	srv        *UService
}

func NewJobManager(queueSize, maxWorkers int, srv *UService) JobManager {
	return JobManager{
		mu: &sync.Mutex{},
		// jobs:       make(map[int]*Job),
		jobQueue:   make(chan Job, queueSize),
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

func (jm *JobManager) ScheduleJob(jb Job) error {
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

func (jm *JobManager) executeJob(job Job) {
	// log.Printf("executing job: %+v", job)
	defer func() { <-jm.workerPool }() // Release worker slot

	jm.mu.Lock()
	job.Status = "running"
	// jm.jobs[job.Jid] = &job
	jm.mu.Unlock()

	// we should examine input "resources"

	// language and version
	cmd, err := performExecution(job, true)
	if err != nil {
		log.Printf("failed to prepare or perform job: %v", err)
		return
	}

	// output should be streamed back ...

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Job %d failed: %s\n", job.Jid, err)
		jm.updateJobStatus(job.Jid, "failed")
		return
	}

	log.Printf("Job %d completed: %s\n", job.Jid, string(output))
	jm.updateJobStatus(job.Jid, "completed")

	// insert the output resource
	jm.syncOutputResource(job)

}

func (jm *JobManager) updateJobStatus(jid int, status string) {
	log.Printf("updating %v job status: %v", jid, status)
	// jm.mu.Lock()
	// defer jm.mu.Unlock()

	// if job, exists := jm.jobs[jid]; exists {
	// 	job.Status = status
	// 	if status == "completed" {
	// 		job.Completed = true
	// 		job.Completed_at = time.Now().Format(time.RFC3339)
	// 	}
	// 	jm.jobs[jid] = job
	// }
	err := jm.srv.dbhJobs.MarkStatus(jid, status)
	if err != nil {
		log.Printf("failed to update job %d status (%s): %v", jid, status, err)
	}

}

func (jm *JobManager) syncOutputResource(job Job) {
	fInfo, err := os.Stat(default_v_path + "/output/" + job.Output)
	if err != nil {
		log.Printf("failed to find/stat the output file: %v", err)
		return
	}

	current_time := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
	resource := Resource{
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

	err = jm.srv.dbh.InsertResource(resource)
	if err != nil {
		log.Printf("failed to insert the resource")
	}
}
