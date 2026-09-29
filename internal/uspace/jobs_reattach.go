package uspace

/*
	Finding jobs again after uspace lost track of them (the database
	dispatcher's reaper, see jobs_dispatcher_db.go): an executor can say
	what its engine knows about a job (inspect) and watch one that is still
	running to its end (reattach), recording the outcome as ExecuteJob does.
*/

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	k "kyri56xcaesar/kuspace/internal/uspace/kubernetes"
	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/minio/minio-go/v7"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// engineState is what an engine knows about a job.
type engineState int

const (
	engineUnknown   engineState = iota // the engine didn't answer: ask again later
	engineRunning                      // still running
	engineSucceeded                    // finished well
	engineFailed                       // finished badly
	engineGone                         // the engine has no such job (never started, or removed)
)

func (s engineState) String() string {
	return [...]string{"unknown", "running", "succeeded", "failed", "gone"}[s]
}

// jobReattacher is what an executor needs for the database dispatcher.
type jobReattacher interface {
	// inspect asks the engine about a job uspace lost track of.
	inspect(ctx context.Context, job ut.Job) (engineState, error)
	// reattach watches a still-running job to its end and records the
	// outcome (status, log, output) as ExecuteJob does. started is when the
	// job started; its timeout counts from there. It takes no worker slot.
	reattach(job ut.Job, started time.Time)
}

// runDeadline bounds a job's run: its timeout (or a day) plus slack, from
// when it started.
func runDeadline(job ut.Job, started time.Time) time.Time {
	limit := 24 * time.Hour
	if job.Timeout > 0 {
		limit = time.Duration(job.Timeout)*time.Minute + 5*time.Minute
	}

	return started.Add(limit)
}

// ---------------------------------------------------------------- docker

// inspect reads the container's state (docker inspect).
func (je JDockerExecutor) inspect(ctx context.Context, job ut.Job) (engineState, error) {
	ctx, cancel := context.WithTimeout(ctx, executorStatusTimeout)
	defer cancel()
	out, err := je.docker(ctx, "inspect", "--format", "{{.State.Status}} {{.State.ExitCode}}", dockerContainerName(job.JID))
	if err != nil {
		if strings.Contains(string(out), "No such object") || strings.Contains(string(out), "No such container") {
			return engineGone, nil
		}

		return engineUnknown, fmt.Errorf("docker inspect: %s", lastLine(out))
	}

	return dockerState(string(out)), nil
}

// dockerState reads docker inspect's "<status> <exit code>".
func dockerState(s string) engineState {
	status, code, _ := strings.Cut(strings.TrimSpace(s), " ")
	switch status {
	case "created", "running", "restarting", "paused":
		return engineRunning
	case "exited", "dead":
		if code == "0" {
			return engineSucceeded
		}

		return engineFailed
	default: // "removing"
		return engineGone
	}
}

// docker runs the docker CLI (je.run in tests).
func (je JDockerExecutor) docker(ctx context.Context, args ...string) ([]byte, error) {
	if je.run != nil {
		return je.run(ctx, args...)
	}

	return exec.CommandContext(ctx, "docker", args...).CombinedOutput()
}

// reattach follows the container's output (docker logs --follow, from the
// start) and waits for it to exit (docker wait). A container that is gone
// before its exit code is read (it runs with --rm) counts as completed when
// it wrote its output.
func (je JDockerExecutor) reattach(job ut.Job, started time.Time) {
	srv := je.rt.srv
	bg := context.Background()
	name := dockerContainerName(job.JID)
	out := newJobOutput(job.JID)
	defer func() {
		if err := srv.saveJobLog(bg, job.JID, out.Close()); err != nil {
			log.Printf("[docker-executor] failed to save the log of job %d: %v", job.JID, err)
		}
	}()
	out.Sendf("[executor] uspace restarted; following %s again\n", name)

	deadline, cancelDeadline := context.WithDeadline(bg, runDeadline(job, started))
	defer cancelDeadline()
	ctx, cancel := context.WithCancelCause(deadline)
	defer cancel(nil)
	untrack := je.rt.trackRunning(job.JID, cancel)
	defer untrack()
	if je.rt.takeCancelled(job.JID) { // cancelled while nobody watched it
		cancel(ErrJobCancelled)
	}

	logsDone := make(chan struct{})
	go func() {
		defer close(logsDone)
		cmd := exec.CommandContext(ctx, "docker", "logs", "--follow", name)
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			return
		}
		cmd.Stderr = cmd.Stdout
		if err := cmd.Start(); err != nil {
			return
		}
		r := bufio.NewReader(pipe)
		for {
			line, err := r.ReadBytes('\n')
			if len(line) > 0 {
				out.Send(append([]byte("\t[CONTAINER]"), line...))
			}
			if err != nil {
				break
			}
		}
		_ = cmd.Wait()
	}()

	waitOut, waitErr := exec.CommandContext(ctx, "docker", "wait", name).CombinedOutput()
	if ctx.Err() != nil { // cancelled or past its deadline: stop the container
		if err := killContainer(job.JID); err != nil {
			log.Printf("[docker-executor] failed to stop %s: %v", name, err)
		}
	}
	select {
	case <-logsDone:
	case <-time.After(10 * time.Second):
	}

	status, reason := "completed", ""
	switch {
	case errors.Is(context.Cause(ctx), ErrJobCancelled):
		status = "cancelled"
	case errors.Is(deadline.Err(), context.DeadlineExceeded):
		status, reason = "failed", "it exceeded its timeout"
	case waitErr != nil:
		// gone before its exit code could be read (--rm)
		if !srv.outputWrittenSince(bg, job, started) {
			status, reason = "failed", "its container is gone and it wrote no output"
		}
	case strings.TrimSpace(string(waitOut)) != "0":
		status, reason = "failed", "its container exited with code "+strings.TrimSpace(string(waitOut))
	}
	srv.finishJob(bg, job, status, reason, time.Since(started), out)
}

// ---------------------------------------------------------------- kubernetes

// inspect reads the kubernetes job's status.
func (jke JKubernetesExecutor) inspect(ctx context.Context, job ut.Job) (engineState, error) {
	client, err := k.GetKubeClient()
	if err != nil {
		return engineUnknown, err
	}

	return k8sState(ctx, client, jke.rt.srv.config.Namespace, job.JID)
}

func k8sState(ctx context.Context, client kubernetes.Interface, ns string, jid int64) (engineState, error) {
	ctx, cancel := context.WithTimeout(ctx, executorStatusTimeout)
	defer cancel()
	j, err := client.BatchV1().Jobs(ns).Get(ctx, k8sJobName(jid), metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		return engineGone, nil
	case err != nil:
		return engineUnknown, err
	}
	for _, c := range j.Status.Conditions {
		if c.Status != "True" {
			continue
		}
		switch c.Type {
		case "Complete":
			return engineSucceeded, nil
		case "Failed":
			return engineFailed, nil
		}
	}

	return engineRunning, nil
}

// reattach watches the kubernetes job again, as ExecuteJob does after
// starting it.
func (jke JKubernetesExecutor) reattach(job ut.Job, started time.Time) {
	srv := jke.rt.srv
	out := newJobOutput(job.JID)
	defer func() {
		if err := srv.saveJobLog(context.Background(), job.JID, out.Close()); err != nil {
			log.Printf("[executor] failed to save the log of job %d: %v", job.JID, err)
		}
	}()
	out.Sendf("[executor] uspace restarted; following %s again\n", k8sJobName(job.JID))
	client, err := k.GetKubeClient()
	if err != nil {
		srv.finishJob(context.Background(), job, "failed", "the cluster can't be reached to follow it: "+err.Error(), time.Since(started), out)

		return
	}
	deadline, cancelDeadline := context.WithDeadline(context.Background(), runDeadline(job, started))
	defer cancelDeadline()
	ctx, cancel := context.WithCancelCause(deadline)
	defer cancel(nil)
	untrack := jke.rt.trackRunning(job.JID, cancel)
	defer untrack()
	if jke.rt.takeCancelled(job.JID) { // cancelled while nobody watched it
		cancel(ErrJobCancelled)
	}
	jke.watch(ctx, client, job, k8sJobName(job.JID), jke.rt.srv.config.Namespace, out, started)
}

// ---------------------------------------------------------------- shared

// finishJob records a job's outcome: its status (with reason when there is
// one to give) and, when it completed, its output.
func (srv *UService) finishJob(ctx context.Context, job ut.Job, status, reason string, d time.Duration, out *jobOutput) {
	msg := "[executor] job " + strconv.FormatInt(job.JID, 10) + " finished with status: " + status
	if reason != "" {
		msg += " (" + reason + ")"
	}
	out.Send([]byte(msg + "\n"))
	if err := srv.markJobStatus(ctx, job.JID, status, d); err != nil {
		log.Printf("failed to mark job %d %s: %v", job.JID, status, err)
	}
	if reason != "" {
		if err := srv.setJobReason(ctx, job.JID, reason); err != nil {
			log.Printf("failed to record why job %d is %s: %v", job.JID, status, err)
		}
	}
	if status == "completed" {
		srv.recordJobOutput(ctx, job, out)
	}
}

// outputWrittenSince reports whether the job's output object was written
// at or after since: how a job whose container is gone shows it finished.
func (srv *UService) outputWrittenSince(ctx context.Context, job ut.Job, since time.Time) bool {
	vname, name, ok := strings.Cut(strings.TrimPrefix(job.Output, "/"), "/")
	if !ok {
		return false
	}
	info, err := srv.storage.Stat(ctx, ut.Resource{Vname: vname, Name: name})
	if err != nil {
		return false
	}
	obj, ok := info.(minio.ObjectInfo)

	return ok && !obj.LastModified.Before(since.Truncate(time.Second))
}
