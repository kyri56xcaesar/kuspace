package uspace

import (
	ut "kyri56xcaesar/kuspace/internal/utils"
)

// JobDispatcher interface description
/*
	Dispatcher for jobs definition
	a system to set job execution in motion, essentially a wrapped scheduler

	this "dispatcher" aspires to be able to connect to a pub/sub system

	the current implementation of this uses a wait Queue and an Executor logic

	@used by the api

	uspace API needs a Job dispatcher.

# This is the interface a dispatcher must implement

@methods:
  - PublishJob(Job) error
  - PublishJobs([]Job) error
  - RemoveJob(int) error
  - RemoveJobs([]int) error

the default one is is JDispatcher which works as a scheduler
*/
type JobDispatcher interface {
	Start()
	PublishJob(ut.Job) error
	PublishJobs([]ut.Job) error
	RemoveJob(int) error
	RemoveJobs([]int) error
	Subscribe(ut.Job) error
}

// DispatcherShipment function for creating (or "shipping") the appropriate Dispatcher
/* a factory contstructor for JobDispatchers: @used by the API*/
func DispatcherShipment(dispatcherType string, srv *UService) (JobDispatcher, error) {
	switch dispatcherType {
	case "scheduler", "default", "local":
		return JobDispatcherImpl{Manager: NewJobManager(srv)}, nil
	case "kafka":
		return nil, ut.NewWarning("kafka dispatcher not implemented")
	case "rabbitmq":
		return nil, ut.NewWarning("rabbitmq dispatcher not implemented")
	case "natss":
		return nil, ut.NewWarning("natss dispatcher not implemented")
	default:
		return nil, ut.NewWarning("unknown dispatcher type: %s", dispatcherType)
	}
}
