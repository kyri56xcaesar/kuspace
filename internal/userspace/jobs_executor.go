package userspace

import ut "kyri56xcaesar/myThesis/internal/utils"

type JobExecutor interface {
	ExecuteJob(job ut.Job) error
	CancelJob(job ut.Job) error
	// GetJobStatus(jobID int) (string, error)
	// GetJobOutput(jobID int) (string, error)
	// GetJobError(jobID int) (string, error)
}

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
