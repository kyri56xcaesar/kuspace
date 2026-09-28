package uspace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	k "kyri56xcaesar/kuspace/internal/uspace/kubernetes"
	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/minio/minio-go/v7"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	duckImage     = "kyri56xcaesar/kuspace:applications-duckdb-v2"
	pandasImage   = "kyri56xcaesar/kuspace:applications-pypandas-v2"
	octaveImage   = "kyri56xcaesar/kuspace:applications-octave-v2"
	ffmpegImage   = "kyri56xcaesar/kuspace:applications-ffmpeg-v2"
	caengineImage = "kyri56xcaesar/kuspace:applications-caengine-v2"
	bashImage     = "kyri56xcaesar/kuspace:applications-bash-v2"
)

// JKubernetesExecutor struct the core data structure impelemnting the JobExecutor interface
// essentially a JobExecutor responsible for executing jobs in cooperation with the Kubernetes API
type JKubernetesExecutor struct {
	jm *JobManager
}

// NewJKubernetesExecutor function as a constructor
func NewJKubernetesExecutor(jm *JobManager) JKubernetesExecutor {
	return JKubernetesExecutor{
		jm: jm,
	}
}

// ExecuteJob method where the core logic of execution happens
func (jke JKubernetesExecutor) ExecuteJob(job ut.Job) error {
	defer func() { <-jke.jm.workerPool }() // release worker slot
	executeK8sJob(&jke, job)

	return nil
}

// k8sJobName is the one name a job has in kubernetes (create, watch, cancel
// and log lookups all use it; they used to disagree).
func k8sJobName(jid int64) string {
	return fmt.Sprintf("job-%d", jid)
}

// CancelJob method responsible for canceling the job execution
func (jke JKubernetesExecutor) CancelJob(job ut.Job) error {
	client, err := k.GetKubeClient()
	if err != nil {
		log.Printf("[executor] could not retrieve k8s client: %v", err)

		return err
	}
	err = cancelJob(client, k8sJobName(job.JID), jke.jm.srv.config.Namespace)
	if err != nil {
		log.Printf("[executor] failed to cancel the Job: %v", err)
	}

	return err
}

// jobQuotas are the parsed resource requests/limits of a job.
type jobQuotas struct {
	reqMem, reqCPU, limMem, limCPU resource.Quantity
}

// parseJobQuotas validates the job's resource strings. resource.MustParse on
// user input used to panic - and a panic in a worker goroutine kills uspace.
func parseJobQuotas(job ut.Job) (jobQuotas, error) {
	var q jobQuotas
	for _, f := range []struct {
		name, val string
		dst       *resource.Quantity
	}{
		{"memory request", job.MemoryRequest, &q.reqMem},
		{"cpu request", job.CPURequest, &q.reqCPU},
		{"memory limit", job.MemoryLimit, &q.limMem},
		{"cpu limit", job.CPULimit, &q.limCPU},
	} {
		v, err := resource.ParseQuantity(f.val)
		if err != nil {
			return q, fmt.Errorf("invalid %s %q: %w", f.name, f.val, err)
		}
		*f.dst = v
	}

	return q, nil
}

func buildK8sJob(
	name string,
	image string,
	command []string,
	env map[string]string,
	quotas jobQuotas,
	parallelism int32,
	namespace string,
	timeout int64,
	ttlSeconds int32,
) *batchv1.Job {
	envVars := []corev1.EnvVar{}
	for k, v := range env {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}
	var deadlinePtr *int64
	if timeout > 0 {
		deadlinePtr = &timeout
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Labels:    map[string]string{"job-group": "uspace-job", "uspace-job": name},
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds:   deadlinePtr,
			Parallelism:             &parallelism,
			Completions:             &parallelism,
			BackoffLimit:            pointerToInt32(0),
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:    "runner",
						Image:   image,
						Command: command,
						Env:     envVars,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: quotas.reqMem,
								corev1.ResourceCPU:    quotas.reqCPU,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: quotas.limMem,
								corev1.ResourceCPU:    quotas.limCPU,
							},
						},
					}},
				},
			},
		},
	}
}

func pointerToInt32(i int32) *int32 {
	return &i
}

func runJob(ctx context.Context, clientset kubernetes.Interface, job *batchv1.Job, namespace string) error {
	_, err := clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})

	return err
}

func cancelJob(clientset kubernetes.Interface, jobName, namespace string) error {
	foreground := metav1.DeletePropagationForeground

	return clientset.BatchV1().Jobs(namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{
		PropagationPolicy: &foreground,
	})
}

// monitorJob waits until the job succeeds or fails. Watches get closed by the
// API server from time to time, so it re-watches until ctx is done; non-Job
// events (errors are *metav1.Status) are skipped - an unchecked type assertion
// here used to panic and take all of uspace down.
func monitorJob(ctx context.Context, clientset kubernetes.Interface, jobName, namespace string) (string, error) {
	for {
		watcher, err := clientset.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
			FieldSelector: "metadata.name=" + jobName,
		})
		if err != nil {
			return "unknown", err
		}
	events:
		for {
			select {
			case <-ctx.Done(): // a silent watch must not outlive the job's deadline
				watcher.Stop()

				return "unknown", fmt.Errorf("stopped watching %s: %w", jobName, ctx.Err())
			case event, open := <-watcher.ResultChan():
				if !open {
					break events
				}
				j, ok := event.Object.(*batchv1.Job)
				if !ok {
					continue
				}
				if j.Status.Succeeded > 0 {
					watcher.Stop()

					return "completed", nil
				}
				if j.Status.Failed > 0 {
					watcher.Stop()

					return "failed", nil
				}
			}
		}
		watcher.Stop()
		select {
		case <-ctx.Done():
			return "unknown", fmt.Errorf("stopped watching %s: %w", jobName, ctx.Err())
		case <-time.After(time.Second):
		}
	}
}

// streamJobLogs follows the logs of the job's first pod once it is running -
// or already finished: short jobs used to complete before ever being "ready",
// so their logs were never streamed.
func streamJobLogs(ctx context.Context, clientset kubernetes.Interface, jobName, namespace string, send func([]byte)) error {
	labelSelector := "job-name=" + jobName // set by kubernetes on the job's pods

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var podName string
	for podName == "" {
		select {
		case <-ctx.Done():
			return fmt.Errorf("no pod started for job %s: %w", jobName, ctx.Err())
		case <-tick.C:
		}
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return fmt.Errorf("error listing pods: %w", err)
		}
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case corev1.PodRunning, corev1.PodSucceeded, corev1.PodFailed:
				podName = pod.Name
			}
		}
	}

	stream, err := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{Follow: true}).Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to open log stream for pod %s: %w", podName, err)
	}
	defer func() {
		if err := stream.Close(); err != nil {
			log.Printf("failed to close the stream: %v", err)
		}
	}()

	reader := bufio.NewReader(stream)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			send(append([]byte("\t[POD]"), line...))
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return fmt.Errorf("error reading log stream: %w", err)
		}
	}
}

// jobLogLimit caps the output kept per job (the tail is kept).
const jobLogLimit = 64 << 10

// jobOutput fans a job's messages out to the live websocket stream and keeps
// the tail for the job's persisted log. Send after Close is a no-op, so a
// straggling goroutine can't panic on a closed channel.
type jobOutput struct {
	mu     sync.Mutex
	ch     chan []byte
	buf    []byte
	closed bool
}

func newJobOutput(jid int64) *jobOutput {
	o := &jobOutput{ch: make(chan []byte, 100)}
	go streamToSocketWS(jid, o.ch)

	return o
}

func (o *jobOutput) Send(msg []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	o.buf = append(o.buf, msg...)
	if len(msg) == 0 || msg[len(msg)-1] != '\n' {
		o.buf = append(o.buf, '\n')
	}
	if len(o.buf) > jobLogLimit {
		o.buf = o.buf[len(o.buf)-jobLogLimit:]
	}
	select {
	case o.ch <- msg:
	default: // the live stream is slow or gone; the persisted log still has it
	}
}

func (o *jobOutput) Sendf(format string, args ...any) { o.Send([]byte(fmt.Sprintf(format, args...))) }

// Close ends the live stream and returns the kept log.
func (o *jobOutput) Close() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.closed {
		o.closed = true
		close(o.ch)
	}

	return string(o.buf)
}

func executeK8sJob(je *JKubernetesExecutor, job ut.Job) {
	jobName := k8sJobName(job.JID)
	namespace := je.jm.srv.config.Namespace
	out := newJobOutput(job.JID)
	defer func() {
		// the log outlives the live stream: save it with the job
		if err := je.jm.srv.saveJobLog(context.Background(), job.JID, out.Close()); err != nil {
			log.Printf("[executor] failed to save the log of job %d: %v", job.JID, err)
		}
	}()

	out.Send([]byte("=-----------------------------------------------------------------------="))
	out.Sendf("[executor] formatting job as %s\n", jobName)

	command, err := je.jm.srv.prepareJobRun(&job)
	if err != nil {
		log.Printf("error formatting job data: %v", err)
		out.Sendf("[executor]: error formatting job data %v\n", err)
		je.markFailed(job.JID, 0)

		return
	}
	quotas, err := parseJobQuotas(job)
	if err != nil {
		out.Sendf("[executor]: %v\n", err)
		je.markFailed(job.JID, 0)

		return
	}

	// safely convert job.Parallelism (int) to int32, checking for overflow
	var parallelism int32
	if job.Parallelism <= 0 || job.Parallelism > math.MaxInt32 {
		parallelism = 1
	} else {
		parallelism = int32(job.Parallelism)
	}

	jobSpec := buildK8sJob(jobName, job.Logic, command, job.Env, quotas, parallelism, namespace,
		int64(job.Timeout*60), je.jm.srv.config.UspaceJobTTL)

	out.Send([]byte("[executor] launching job...\n"))
	out.Sendf("[executor] specs: {parallelism: %v, timeout: %v, cpu_limit: %v, cpu_request: %v, mem_limit: %v, mem_req: %v}\n",
		job.Parallelism, job.Timeout, job.CPULimit, job.CPURequest, job.MemoryLimit, job.MemoryRequest)

	clientset, err := k.GetKubeClient() // from config
	if err != nil {
		log.Printf("[executor] could not retrieve kube client: %v", err)
		out.Send([]byte("could not retrieve k8s client, fatal...\nexiting..."))
		je.markFailed(job.JID, 0)

		return
	}

	// the whole run is bounded: the job's own timeout (or a day) plus slack
	limit := 24 * time.Hour
	if job.Timeout > 0 {
		limit = time.Duration(job.Timeout)*time.Minute + 5*time.Minute
	}
	deadline, cancelDeadline := context.WithTimeout(context.Background(), limit)
	defer cancelDeadline()
	// a user's cancel (JobManager.CancelJob) cancels this with ErrJobCancelled
	ctx, cancel := context.WithCancelCause(deadline)
	defer cancel(nil)
	untrack := je.jm.trackRunning(job.JID, cancel)
	defer untrack()
	if je.jm.takeCancelled(job.JID) { // cancelled between dequeue and here
		cancel(ErrJobCancelled)
	}

	if err := runJob(ctx, clientset, jobSpec, namespace); err != nil {
		log.Printf("error starting job: %v", err)
		out.Sendf("[executor]: error launching job execution %v\n", err)
		je.markFailed(job.JID, 0)

		return
	}
	startTime := time.Now()
	if err := je.jm.srv.markJobStatus(context.Background(), job.JID, "running", 0); err != nil {
		log.Printf("failed to mark job %d running: %v", job.JID, err)
	}
	if err := je.jm.srv.markJobEngine(context.Background(), job.JID, "kubernetes"); err != nil {
		log.Printf("failed to record job %d engine: %v", job.JID, err)
	}

	var logsDone sync.WaitGroup
	logsDone.Add(1)
	go func() {
		defer logsDone.Done()
		if err := streamJobLogs(ctx, clientset, jobName, namespace, out.Send); err != nil {
			log.Printf("failed to stream job logs: %v", err)
			out.Sendf("[executor]: error streaming pod logs: %v\n", err)
		}
	}()

	status, err := monitorJob(ctx, clientset, jobSpec.Name, namespace)
	if err != nil {
		log.Printf("error monitoring job: %v", err)
	}
	if errors.Is(context.Cause(ctx), ErrJobCancelled) {
		// stop the pod: deleting the kubernetes job (foreground) removes it
		if err := cancelJob(clientset, jobSpec.Name, namespace); err != nil {
			log.Printf("failed to delete cancelled job %s: %v", jobSpec.Name, err)
		}
		status = "cancelled"
		out.Send([]byte("[executor] job cancelled by its owner\n"))
	}
	// give the log follower a moment to drain what the pod printed last
	drained := make(chan struct{})
	go func() { logsDone.Wait(); close(drained) }()
	select {
	case <-drained:
	case <-time.After(10 * time.Second):
	}

	duration := time.Since(startTime)
	out.Sendf("[executor] job %v finished with status: %s, duration: %v\n", jobName, status, duration)

	if err := je.jm.srv.markJobStatus(context.Background(), job.JID, status, duration); err != nil {
		log.Printf("failed to annotate result to database")
		out.Sendf("[executor]: error marking job completion %v\n", err)
	}

	if status == "completed" {
		je.jm.srv.recordJobOutput(context.Background(), job, out)
	}
}

func (je *JKubernetesExecutor) markFailed(jid int64, d time.Duration) {
	if err := je.jm.srv.markJobStatus(context.Background(), jid, "failed", d); err != nil {
		log.Printf("failed to mark job %d failed: %v", jid, err)
	}
}

// recordJobOutput stats a finished job's output object and records it
// (saveJobOutput). Shared by the executors.
// recordJobOutput runs after the job, so ctx is not a request context.
func (srv *UService) recordJobOutput(ctx context.Context, job ut.Job, out *jobOutput) {
	vname, name, ok := strings.Cut(job.Output, "/")
	if !ok {
		out.Send([]byte("[executor]: invalid output location\n"))

		return
	}
	outputResource := ut.Resource{
		Name:  name,
		Path:  "/",
		Type:  "file",
		Perms: ut.DefaultFilePerms,
		UID:   job.UID,
		Vname: vname,
		VID:   srv.volumeID(ctx, vname),
		GID:   jobGID(job),
	}
	info, err := srv.storage.Stat(ctx, outputResource)
	if err != nil {
		log.Printf("failed to stat output file from storage: %v", err)
		out.Sendf("[executor]: error retrieving output file %v\n", err)

		return
	}
	// this should be changed to be independent of minio... // will do "resourceInfo struct "
	infoCasted, ok := info.(minio.ObjectInfo)
	if !ok {
		out.Send([]byte("[executor]: error retrieving output file format\n"))

		return
	}
	outputResource.Size = infoCasted.Size
	now := ut.CurrentTime()
	outputResource.CreatedAt, outputResource.UpdatedAt, outputResource.AccessedAt = now, now, now

	out.Sendf("[executor] saving output %s/%s ...\n", outputResource.Vname, outputResource.Name)
	action, err := srv.saveJobOutput(ctx, outputResource)
	if err != nil {
		log.Printf("failed to record output of job %d: %v", job.JID, err)
		out.Sendf("[executor]: error saving output data in db... %v\n", err)

		return
	}
	out.Sendf("[executor] output %s\n", action)
	out.Send([]byte("[executor] OK.\n"))
}

// prepareJobRun fills in a job's defaults, image, command and environment
// (including its presigned input/output URLs). Shared by the executors.
func (srv *UService) prepareJobRun(job *ut.Job) ([]string, error) {
	// handle some generic checks as guard statement
	if !ut.AssertStructNotEmptyUpon(job, map[any]bool{
		"Input":     true,
		"Output":    true,
		"Logic":     true,
		"LogicBody": true,
	}) {
		return nil, ut.NewError("empty field that shouldn't be empty..")
	}

	// Assuming job.Logic is the image name and job.LogicBody is the command
	var (
		InpAsResource ut.Resource
		OutAsResource ut.Resource
	)
	command, err := formatJobCommand(job)
	if err != nil {
		log.Printf("error formatting job command: %v", err)

		return nil, err
	}

	// create an env map
	envMap := make(map[string]string)

	// inp/out can be in format <volume>/<path>

	// we should handle the job input by contacting the storage_system api
	// can do it with Share or simply by Stat the objects, idk
	parts := strings.Split(job.Input, "/")
	if len(parts) > 1 {
		InpAsResource.Vname = parts[0]
		InpAsResource.Name = strings.Join(parts[1:], "/")
	} else {
		InpAsResource.Vname = srv.storage.DefaultVolume(false)
		InpAsResource.Name = job.Input
	}

	parts = strings.Split(job.Output, "/")
	if len(parts) > 1 {
		OutAsResource.Vname = parts[0]
		OutAsResource.Name = strings.Join(parts[1:], "/")
	} else {
		OutAsResource.Vname = srv.storage.DefaultVolume(false)
		OutAsResource.Name = job.Output
	}

	// format job vars
	job.Output = strings.TrimSpace(job.Output)
	job.OutputFormat = strings.TrimSpace(job.OutputFormat)
	if job.OutputFormat == "" {
		p := strings.Split(job.Input, ".")
		if len(p) == 0 || p[len(p)-1] == "" {
			job.OutputFormat = "txt" // default format
		} else {
			job.OutputFormat = p[len(p)-1]
		}
	}
	if job.InputFormat == "" {
		// deduce input format
		p := strings.Split(job.Input, ".")
		if len(p) == 0 || p[len(p)-1] == "" {
			job.InputFormat = "txt" // default format
		} else {
			job.InputFormat = p[len(p)-1]
		}
	}
	if job.Parallelism == 0 { // default parallelism
		job.Parallelism = 1
	}

	if job.MemoryLimit == "" {
		job.MemoryLimit = "4Gi" // default limit
	}

	if job.CPULimit == "" {
		job.CPULimit = "1000m"
	}

	if job.MemoryRequest == "" {
		job.MemoryRequest = "2Gi"
	}

	if job.CPURequest == "" {
		job.CPURequest = "500m"
	}

	// The pod runs user code, so it gets no storage credentials: only
	// presigned URLs for exactly its input (GET) and output (PUT), valid
	// while the job may run. (It used to get the MinIO root keys - any job
	// could read or overwrite every user's files.)
	inputURL, outputURL, err := presignJobIO(srv.storage, InpAsResource, OutAsResource, jobURLValidity(job.Timeout))
	if err != nil {
		return nil, err
	}
	envMap["INPUT_URL"] = inputURL
	envMap["OUTPUT_URL"] = outputURL
	envMap["LOGIC"] = job.LogicBody
	envMap["INPUT_BUCKET"] = InpAsResource.Vname
	envMap["INPUT_OBJECT"] = InpAsResource.Name
	envMap["INPUT_FORMAT"] = job.InputFormat
	envMap["OUTPUT_BUCKET"] = OutAsResource.Vname
	envMap["OUTPUT_OBJECT"] = OutAsResource.Name
	envMap["OUTPUT_FORMAT"] = job.OutputFormat
	envMap["TIMEOUT"] = strconv.Itoa(job.Timeout)
	job.Env = envMap

	return command, nil
}

// presigner is implemented by storage backends that can hand out
// per-object URLs (MinIO).
type presigner interface {
	PresignFor(ctx context.Context, method string, r ut.Resource, d time.Duration) (*url.URL, error)
}

// jobURLValidity is how long a job's input/output URLs stay valid: its
// timeout plus slack for scheduling, or 6 hours for jobs without one.
func jobURLValidity(timeoutMinutes int) time.Duration {
	if timeoutMinutes > 0 {
		return time.Duration(timeoutMinutes)*time.Minute + 15*time.Minute
	}

	return 6 * time.Hour
}

func presignJobIO(storage any, in, out ut.Resource, valid time.Duration) (string, string, error) {
	p, ok := storage.(presigner)
	if !ok {
		return "", "", errors.New("the storage backend can't issue per-object URLs; jobs need MinIO")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	get, err := p.PresignFor(ctx, "get", in, valid)
	if err != nil {
		return "", "", fmt.Errorf("presign input: %w", err)
	}
	put, err := p.PresignFor(ctx, "put", out, valid)
	if err != nil {
		return "", "", fmt.Errorf("presign output: %w", err)
	}

	return get.String(), put.String(), nil
}

// codeMode runs a job's code (in $LOGIC) with a language's public image.
// The code gets INPUT_URL/OUTPUT_URL like the applications but does its own
// input and output.
type codeMode struct {
	image string // docker image repository
	tag   string // default tag (small images); a job's "lang:tag" overrides it
	run   string // sh command; the code is in $LOGIC
	desc  string // shown in the app catalogue
}

var (
	codeModes = map[string]codeMode{
		"python": {"python", "3.12-alpine", `python3 -c "$LOGIC"`, "Python 3 code"},
		"node":   {"node", "22-alpine", `node -e "$LOGIC"`, "JavaScript on Node.js"},
		"ruby":   {"ruby", "3.3-alpine", `ruby -e "$LOGIC"`, "Ruby code"},
		"php":    {"php", "8.3-cli-alpine", `php -r "$LOGIC"`, "PHP code (CLI)"},
		"perl":   {"perl", "5.40-slim", `perl -e "$LOGIC"`, "Perl 5 code"},
		"r":      {"r-base", "4.4.2", `Rscript -e "$LOGIC"`, "R code (Rscript)"},
		"go":     {"golang", "1.24-alpine", `printf '%s' "$LOGIC" > /tmp/main.go && go run /tmp/main.go`, "A Go program (package main)"},
		"java":   {"eclipse-temurin", "21-jdk-alpine", `printf '%s' "$LOGIC" > /tmp/Main.java && java /tmp/Main.java`, "A Java program (class Main)"},
		"c":      {"gcc", "14", `printf '%s' "$LOGIC" > /tmp/main.c && gcc -o /tmp/main /tmp/main.c && /tmp/main`, "A C program (gcc)"},
	}
	codeAliases = map[string]string{
		"py": "python", "javascript": "node", "js": "node", "golang": "go",
		"javac": "java", "openjdk": "java", "gcc": "c",
	}
)

func supportedLanguages() string {
	names := make([]string, 0, len(codeModes))
	for n := range codeModes {
		names = append(names, n)
	}
	sort.Strings(names)

	return strings.Join(names, ", ")
}

func formatJobCommand(job *ut.Job) ([]string, error) {
	var name, version string
	// deduct name and version and format it
	p := strings.Split(strings.TrimSpace(job.Logic), ":")
	explicit := len(p) == 2
	if explicit {
		name = p[0]
		version = p[1]
	} else {
		name = p[0]
		version = "latest"
	}
	if name == "" || version == "" {
		return nil, errors.New("invalid job data")
	}
	job.Logic = fmt.Sprintf("%s:%s", name, version)

	lang := job.Logic[:strings.Index(job.Logic+":", ":")]
	switch lang {
	case "application/duckdb", "duckdb": // check if the given logic is a custom app
		job.Logic = duckImage

		return []string{"python", "duckdb_app.py"}, nil
	case "application/pypandas", "pandas", "pypandas":
		job.Logic = pandasImage

		return []string{"python", "pypandas_app.py"}, nil
	case "application/octave", "octave":
		job.Logic = octaveImage

		return []string{"python3", "octave_app.py"}, nil
	case "application/ffmpeg", "ffmpeg":
		job.Logic = ffmpegImage

		return []string{"python3", "ffmpeg_app.py"}, nil
	case "application/caengine", "caengine":
		job.Logic = caengineImage

		return []string{"python3", "caengine_app.py"}, nil
	case "application/bash", "bash", "sh", "shell":
		job.Logic = bashImage

		return []string{"python3", "bash_app.py"}, nil
	default:
		mode, ok := codeModes[codeAliases[lang]]
		if !ok {
			mode, ok = codeModes[lang]
		}
		if !ok {
			return nil, fmt.Errorf("unsupported application or language %q (languages: %s)", lang, supportedLanguages())
		}
		if !explicit { // the language's small default image, not :latest
			version = mode.tag
		}
		job.Logic = mode.image + ":" + version
		// the code reaches the interpreter through $LOGIC (job env), never
		// pasted into the command: quotes in the code used to break it
		return []string{"/bin/sh", "-c", mode.run}, nil
	}
}

// saveJobOutput records a job's output object: a new one is created (owned
// by the job's owner); an existing one (a job may overwrite an output its
// owner can write, checked at submission) gets its new size and time. Usage
// follows the records, so nothing else is charged or refunded.
func (srv *UService) saveJobOutput(ctx context.Context, output ut.Resource) (string, error) {
	quota := min(srv.config.LocalVolumesDefaultCapacity, maxDefaultVolumeCapacity)
	_, found, err := srv.lookupResource(ctx, output.Name, output.Vname)
	if err != nil {
		return "", err
	}
	if !found {
		// recorded even past the quota: the job already wrote it
		if err := srv.fsl.InsertResource(ctx, output, quota, false); err != nil {
			return "", err
		}

		return "created", nil
	}

	if err := srv.fsl.SetObjectSize(ctx, output.Name, output.Vname, output.Size); err != nil {
		return "", err
	}

	return "updated (overwrote the existing file)", nil
}

// jobGID is the group a job's outputs belong to: the owner's primary group
// recorded at submission (older jobs: the owner's uid).
func jobGID(job ut.Job) int64 {
	if job.GID > 0 {
		return job.GID
	}

	return job.UID
}
