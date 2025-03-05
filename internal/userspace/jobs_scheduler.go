package userspace

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
type JobScheduler struct{}

func (j JobScheduler) ScheduleJob(jb Job) error {
	return nil
}

func (j JobScheduler) CancelJob(jid int) error {
	return nil
}

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
