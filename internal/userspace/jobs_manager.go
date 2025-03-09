package userspace

import (
	"fmt"
	"log"
	"os"
	"os/exec"
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
	log.Printf("publishing job... :%v", jb)
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
	mu         *sync.Mutex
	jobs       map[int]*Job
	jobQueue   chan Job
	workerPool chan struct{}
}

func NewJobManager(queueSize, maxWorkers int) JobManager {
	return JobManager{
		mu:         &sync.Mutex{},
		jobs:       make(map[int]*Job),
		jobQueue:   make(chan Job, queueSize),
		workerPool: make(chan struct{}, maxWorkers),
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
	if _, exists := jm.jobs[jb.Jid]; exists {
		return fmt.Errorf("job %d already exists", jb.Jid)
	}

	jb.Status = "queued"
	jb.Created_at = time.Now().Format(time.RFC3339)
	jm.jobs[jb.Jid] = &jb
	jm.jobQueue <- jb

	log.Printf("Job %d submitted\n", jb.Jid)
	return nil
}

func (js *JobManager) CancelJob(jid int) error {
	log.Printf("canceling job: %v", jid)
	js.mu.Lock()
	defer js.mu.Unlock()

	if _, exists := js.jobs[jid]; !exists {
		return fmt.Errorf("job %d not found", jid)
	}

	delete(js.jobs, jid)
	log.Printf("Job %d cancelled\n", jid)
	return nil
}

func (jm *JobManager) executeJob(job Job) {
	log.Printf("executing job: %+v", job)
	defer func() { <-jm.workerPool }() // Release worker slot

	jm.mu.Lock()
	job.Status = "running"
	jm.jobs[job.Jid] = &job
	jm.mu.Unlock()

	// Save user logic to a script file
	scriptFile := fmt.Sprintf("tmp/job-%d.py", job.Jid)
	err := os.WriteFile(scriptFile, []byte(job.LogicBody), 0644)
	if err != nil {
		log.Printf("Failed to write script: %s\n", err)
		jm.updateJobStatus(job.Jid, "failed")
		return
	}

	// Execute job inside a Docker container, passing input/output format
	// cmd := exec.Command("docker", "run", "--rm",
	// 	"-v", fmt.Sprintf("%s:/app/script.py", scriptFile),
	// 	"-v", fmt.Sprintf("%s:/app/input", job.Input),
	// 	"-v", fmt.Sprintf("%s:/app/output", job.Output),
	// 	"python:3.9", "python", "-c", fmt.Sprintf(`
	// 	import sys
	// 	from format_handler import load_input, save_output
	// 	from script import run

	// 	input_format = "%s"
	// 	output_format = "%s"

	// 	# Load input data based on format
	// 	data = load_input("/app/input", input_format)

	// 	# Execute the user's function
	// 	result = run(data)

	// 	# Save output based on format
	// 	save_output(result, "/app/output", output_format)
	// 	`, job.InputFormat, job.OutputFormat))
	cmd := exec.Command("docker", "run", "--rm", "python:3.9", "python", "-c", "print('hello from inside')")

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Job %d failed: %s\n", job.Jid, err)
		jm.updateJobStatus(job.Jid, "failed")
		return
	}

	log.Printf("Job %d completed: %s\n", job.Jid, string(output))
	jm.updateJobStatus(job.Jid, "completed")
}

func (js *JobManager) updateJobStatus(jid int, status string) {
	log.Printf("updating %v job status: %v", jid, status)
	js.mu.Lock()
	defer js.mu.Unlock()

	if job, exists := js.jobs[jid]; exists {
		job.Status = status
		if status == "completed" {
			job.Completed = true
			job.Completed_at = time.Now().Format(time.RFC3339)
		}
		js.jobs[jid] = job
	}
}
