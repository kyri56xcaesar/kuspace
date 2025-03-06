package userspace

import(
	"sync"
)

type JDispatcher struct {
	Scheduler JobScheduler
}

/*
a Job scheduler is the default implementation for publishing/dispatching Jobs

@alternatives:
  - a broker

@methods:
  - ScheduleJob(Job) error
  - CancelJob(Job) error
*/
type JobScheduler struct{
	mu *sync.Mutex
	jobs map[int]*Job 
	jobQueue chan Job
}

func NewJobScheduler(queueSize int) *JobScheduler {
	return  &JobScheduler{
		jobs: make(map[int]*Job)
		jobQueue: make(chan Job, queueSize)
	}
}

func (js *JobScheduler) ScheduleJob(jb Job) error {
	js.mu.Lock()
	defer js.mu.Unlock()

	return nil
}

func (js *JobScheduler) CancelJob(jid int) error {
	return nil
}



/* dispatching jobs interface methods */
func (j JDispatcher) PublishJob(jb Job) error {
	return j.Scheduler.ScheduleJob(jb)
}

func (j JDispatcher) PublishJobs(jbs []Job) error {
	for _, jb := range jbs {
		err := j.Scheduler.ScheduleJob(jb)
		if err != nil {
			return err
		}
	}
	return nil
}

func (j JDispatcher) RemoveJob(jid int) error {
	return j.Scheduler.CancelJob(jid)
}

func (j JDispatcher) RemoveJobs(jids []int) error {
	for _, jid := range jids {
		err := j.Scheduler.CancelJob(jid)
		if err != nil {
			return err
		}
	}
	return nil
}
