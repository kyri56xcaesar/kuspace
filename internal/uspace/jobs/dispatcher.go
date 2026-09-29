package jobs

import (
	"context"
	"errors"
	"fmt"

	"kyri56xcaesar/kuspace/internal/model"
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
	PublishJob(job model.Job) error
	PublishJobs(jobs []model.Job) error
	RemoveJob(jid int) error
	RemoveJobs(jids []int) error
	Subscribe(job model.Job) error
	// Engines are the execution engines and the one taking new jobs.
	Engines() *EngineSet
}

// New builds the dispatcher J_DISPATCHER names, on a runtime with both engines.
func New(dispatcherType string, deps Deps) (JobDispatcher, error) {
	switch dispatcherType {
	case "scheduler", "default", "local":

		return JobDispatcherImpl{Manager: NewJobManager(deps)}, nil
	case "database", "db":
		// the jobs table is the queue: jobs survive restarts (jobs_dispatcher_db.go)
		return NewDBDispatcher(newJobRuntime(deps), maxWorkers(deps.Config))
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

// NewWithEngines is the in-memory dispatcher over the given engines (the
// first takes new jobs): for callers that bring their own, such as tests.
func NewWithEngines(deps Deps, executors ...JobExecutor) JobDispatcher {
	return JobDispatcherImpl{Manager: newJobManagerOn(newRuntimeWith(deps, executors...))}
}
