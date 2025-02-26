package main

import (
	"fmt"
	"sync"
	"time"
)

// Job struct
type Job struct {
	ID        string
	Payload   string
	Status    string
	Retries   int
	CreatedAt time.Time
}

// Job Queue
type JobQueue struct {
	jobs   chan Job
	mutex  sync.Mutex
	active map[string]Job
}

// NewJobQueue initializes a job queue
func NewJobQueue(size int) *JobQueue {
	return &JobQueue{
		jobs:   make(chan Job, size),
		active: make(map[string]Job),
	}
}

// Publish job to the queue
func (q *JobQueue) Publish(job Job) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	job.Status = "queued"
	q.jobs <- job
	fmt.Println("Published job:", job.ID)
}

// Subscribe (Worker) to process jobs
func (q *JobQueue) Subscribe(workerID int) {
	for job := range q.jobs {
		q.mutex.Lock()
		job.Status = "running"
		q.active[job.ID] = job
		q.mutex.Unlock()

		fmt.Printf("Worker %d processing job %s\n", workerID, job.ID)
		time.Sleep(2 * time.Second) // Simulating work

		q.mutex.Lock()
		job.Status = "completed"
		delete(q.active, job.ID)
		q.mutex.Unlock()

		fmt.Printf("Worker %d completed job %s\n", workerID, job.ID)
	}
}

func main() {
	queue := NewJobQueue(10)

	// Start workers
	for i := 1; i <= 3; i++ {
		go queue.Subscribe(i)
	}

	// Publish jobs
	for i := 1; i <= 5; i++ {
		job := Job{
			ID:        fmt.Sprintf("job-%d", i),
			Payload:   fmt.Sprintf("Task %d", i),
			CreatedAt: time.Now(),
		}
		queue.Publish(job)
	}

	// Keep the main thread alive
	time.Sleep(10 * time.Second)
}
