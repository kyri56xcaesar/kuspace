// Package uspace provides job execution interfaces and factory functions for different execution engines.
//
//	define how to execute a job in an container engine (and more)
//
// examples: (already defined)
// - docker engine
// - kubernetes engine
package uspace

import (
	"context"
	"fmt"
	"strings"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

// JobExecutor interface defining what a JobExecutor must implement
type JobExecutor interface {
	ExecuteJob(job ut.Job) error
	CancelJob(job ut.Job) error
	// Status checks whether the engine answers and can start jobs.
	Status(ctx context.Context) ut.ExecutorStatus
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

		return nil, fmt.Errorf("unknown job executor (%w)", ut.ErrInvalid)
	}
}

// executorStatusTimeout bounds an engine check.
const executorStatusTimeout = 5 * time.Second

// lastLine is the last non-empty line of a command's output (where the CLI
// tools put the error).
func lastLine(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	return strings.TrimSpace(lines[len(lines)-1])
}
