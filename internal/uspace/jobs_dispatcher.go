package uspace

import (
	"context"
	"errors"
	"fmt"
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

the default one is JDispatcher which works as a scheduler
*/
type JobDispatcher interface {
	Start()
	Drain(ctx context.Context) error
	PublishJob(job ut.Job) error
	PublishJobs(jobs []ut.Job) error
	RemoveJob(jid int) error
	RemoveJobs(jids []int) error
	Subscribe(job ut.Job) error
}

// DispatcherShipment function for creating (or "shipping") the appropriate Dispatcher
/* a factory contstructor for JobDispatchers: @used by the API*/
func DispatcherShipment(dispatcherType string, srv *UService) (JobDispatcher, error) {
	switch dispatcherType {
	case "scheduler", "default", "local":

		return JobDispatcherImpl{Manager: NewJobManager(srv)}, nil
	case "kafka":

		return nil, errors.New("the kafka dispatcher is not implemented")
	case "rabbitmq":

		return nil, errors.New("the rabbitmq dispatcher is not implemented")
	case "natss":

		return nil, errors.New("the nats dispatcher is not implemented")
	default:

		return nil, fmt.Errorf("unknown dispatcher type %q", dispatcherType)
	}
}
