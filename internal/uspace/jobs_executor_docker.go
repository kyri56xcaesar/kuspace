package uspace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sort"
	"strconv"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

/*
	JDockerExecutor runs jobs as containers on the host's docker engine - for
	development and debugging without a kubernetes cluster (J_EXECUTOR=docker).

	It keeps the kubernetes executor's contract: the same application images,
	only presigned INPUT_URL/OUTPUT_URL (no storage credentials), live output
	plus a saved log, cancellation, output recording and quotas. Containers
	join J_DOCKER_NETWORK, so they reach MinIO under the host name the URLs
	were signed for.

	uspace needs the docker CLI and the engine's socket for this, which is as
	good as root on that host: development only.
*/

// JDockerExecutor struct implementing the JobExecutor interface
type JDockerExecutor struct {
	jm *JobManager
}

// NewJDockerExecutor function as a constructor
func NewJDockerExecutor(jm *JobManager) JDockerExecutor {
	return JDockerExecutor{jm: jm}
}

func dockerContainerName(jid int64) string {
	return fmt.Sprintf("kuspace-job-%d", jid)
}

// dockerRunArgs builds the `docker run` arguments for a prepared job
// (image in job.Logic, environment in job.Env).
func dockerRunArgs(job ut.Job, command []string, q jobQuotas, network string) []string {
	args := []string{
		"run", "--rm",
		"--name", dockerContainerName(job.JID),
		"--label", "kuspace.job=" + strconv.FormatInt(job.JID, 10),
		"--memory", strconv.FormatInt(q.limMem.Value(), 10),
		"--cpus", strconv.FormatFloat(float64(q.limCPU.MilliValue())/1000, 'f', 3, 64),
		"--pids-limit", "512",
		"--security-opt", "no-new-privileges",
	}
	if network != "" {
		args = append(args, "--network", network)
	}
	keys := make([]string, 0, len(job.Env))
	for k := range job.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", k+"="+job.Env[k])
	}
	args = append(args, job.Logic) // the image

	return append(args, command...)
}

func killContainer(jid int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return exec.CommandContext(ctx, "docker", "kill", dockerContainerName(jid)).Run()
}

// ExecuteJob method, the core logic of execution
func (je JDockerExecutor) ExecuteJob(job ut.Job) error {
	defer func() { <-je.jm.workerPool }() // release worker slot
	srv := je.jm.srv
	bg := context.Background()

	out := newJobOutput(job.JID)
	defer func() {
		// the log outlives the live stream: save it with the job
		if err := srv.saveJobLog(bg, job.JID, out.Close()); err != nil {
			log.Printf("[docker-executor] failed to save the log of job %d: %v", job.JID, err)
		}
	}()
	fail := func(err error) error {
		log.Printf("[docker-executor] job %d: %v", job.JID, err)
		out.Sendf("[executor]: %v\n", err)
		if mErr := srv.markJobStatus(bg, job.JID, "failed", 0); mErr != nil {
			log.Printf("[docker-executor] failed to mark job %d failed: %v", job.JID, mErr)
		}

		return err
	}

	out.Send([]byte("=-----------------------------------------------------------------------="))
	command, err := srv.prepareJobRun(&job)
	if err != nil {
		return fail(fmt.Errorf("error formatting job data: %w", err))
	}
	quotas, err := parseJobQuotas(job)
	if err != nil {
		return fail(err)
	}

	limit := 24 * time.Hour
	if job.Timeout > 0 {
		limit = time.Duration(job.Timeout)*time.Minute + 5*time.Minute
	}
	deadline, cancelDeadline := context.WithTimeout(bg, limit)
	defer cancelDeadline()
	ctx, cancel := context.WithCancelCause(deadline)
	defer cancel(nil)
	untrack := je.jm.trackRunning(job.JID, cancel)
	defer untrack()
	if je.jm.takeCancelled(job.JID) { // cancelled between dequeue and here
		cancel(ErrJobCancelled)
	}

	cmd := exec.CommandContext(ctx, "docker", dockerRunArgs(job, command, quotas, srv.config.UspaceJobDockerNetwork)...)
	// on cancel/deadline stop the container itself, not just the docker CLI
	cmd.Cancel = func() error { return killContainer(job.JID) }
	cmd.WaitDelay = 15 * time.Second
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	streamed := make(chan struct{})
	go func() {
		defer close(streamed)
		reader := bufio.NewReader(pr)
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				out.Send(append([]byte("\t[CONTAINER]"), line...))
			}
			if err != nil {
				return
			}
		}
	}()

	out.Sendf("[executor] running %s on docker (%s)\n", job.Logic, dockerContainerName(job.JID))
	start := time.Now()
	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		<-streamed

		return fail(fmt.Errorf("failed to start the container: %w", err))
	}
	if err := srv.markJobStatus(bg, job.JID, "running", 0); err != nil {
		log.Printf("[docker-executor] failed to mark job %d running: %v", job.JID, err)
	}
	if err := srv.markJobEngine(bg, job.JID, "docker"); err != nil {
		log.Printf("[docker-executor] failed to record job %d engine: %v", job.JID, err)
	}

	runErr := cmd.Wait()
	_ = pw.Close()
	<-streamed
	duration := time.Since(start)

	status := "completed"
	switch {
	case errors.Is(context.Cause(ctx), ErrJobCancelled):
		status = "cancelled"
		out.Send([]byte("[executor] job cancelled by its owner\n"))
	case errors.Is(deadline.Err(), context.DeadlineExceeded):
		status = "failed"
		out.Send([]byte("[executor] job exceeded its timeout\n"))
	case runErr != nil:
		status = "failed"
		out.Sendf("[executor] container failed: %v\n", runErr)
	}
	out.Sendf("[executor] job %d finished with status: %s, duration: %v\n", job.JID, status, duration)
	if err := srv.markJobStatus(bg, job.JID, status, duration); err != nil {
		log.Printf("[docker-executor] failed to mark job %d %s: %v", job.JID, status, err)
	}
	if status == "completed" {
		srv.recordJobOutput(job, out)
	}

	return nil
}

// CancelJob stops the job's container.
func (je JDockerExecutor) CancelJob(job ut.Job) error {
	return killContainer(job.JID)
}
