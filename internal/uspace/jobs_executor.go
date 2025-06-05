// Package uspace provides job execution interfaces and factory functions for different execution engines.
//
//	define how to execute a job in an container engine (and more)
//
// examples: (already defined)
// - docker engine
// - kubernetes engine
package uspace

import ut "kyri56xcaesar/kuspace/internal/utils"

// JobExecutor interface defining what a JobExecutor must implement
type JobExecutor interface {
	ExecuteJob(job ut.Job) error
	CancelJob(job ut.Job) error
	// GetJobStatus(jobID int) (string, error)
	// GetJobOutput(jobID int) (string, error)
	// GetJobError(jobID int) (string, error)
}

// JobExecutorShipment "ships"/returns the JobExecutor asked
func JobExecutorShipment(jobType string, jm *JobManager) (JobExecutor, error) {
	switch jobType {
	case "local", "docker", "default":
		return NewJDockerExecutor(jm), nil
	case "kubernetes":
		return NewJKubernetesExecutor(jm), nil
	default:
		return nil, ut.NewError("Invalid job type")
	}
}
