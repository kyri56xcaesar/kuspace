package userspace

import "fmt"

/*
	Dispatcher for jobs

	@used by the api
*/

/*
	Userspace API needs a Job dispatcher.

# This is the interface a dispatcher must implement

@methods:
  - PublishJob(Job) error
  - PublishJobs([]Job) error
  - RemoveJob(int) error
  - RemoveJobs([]int) error

the default one is is JDispatcher which works as a scheduler
*/
type JobDispatcher interface {
	PublishJob(Job) error
	PublishJobs([]Job) error
	RemoveJob(int) error
	RemoveJobs([]int) error
}

/* a factory contstructor for JobDispatchers: @used by the API*/
func DispatcherFactory(dispatcherType string) (JobDispatcher, error) {
	switch dispatcherType {
	case "scheduler", "default", "local":
		return JDispatcher{Scheduler: NewJobScheduler(100)}, nil
	case "kafka":
		return nil, fmt.Errorf("kafka dispatcher not implemented")
	case "rabbitmq":
		return nil, fmt.Errorf("rabbitmq dispatcher not implemented")
	case "natss":
		return nil, fmt.Errorf("natss dispatcher not implemented")
	default:
		return nil, fmt.Errorf("unknown dispatcher type: %s", dispatcherType)
	}
}
