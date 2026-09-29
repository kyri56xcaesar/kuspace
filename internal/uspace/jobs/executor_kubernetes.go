package jobs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	k "kyri56xcaesar/kuspace/internal/uspace/kubernetes"
	ut "kyri56xcaesar/kuspace/internal/utils"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"kyri56xcaesar/kuspace/internal/model"
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
	rt *jobRuntime
}

// NewJKubernetesExecutor function as a constructor
func NewJKubernetesExecutor(rt *jobRuntime) JKubernetesExecutor {
	return JKubernetesExecutor{
		rt: rt,
	}
}

// ExecuteJob method where the core logic of execution happens
func (jke JKubernetesExecutor) ExecuteJob(job model.Job) error {
	executeK8sJob(&jke, job)

	return nil
}

// k8sJobName is the one name a job has in kubernetes (create, watch, cancel
// and log lookups all use it; they used to disagree).
func k8sJobName(jid int64) string {
	return fmt.Sprintf("job-%d", jid)
}

// CancelJob method responsible for canceling the job execution
func (jke JKubernetesExecutor) CancelJob(job model.Job) error {
	client, err := k.GetKubeClient()
	if err != nil {
		log.Printf("[executor] could not retrieve k8s client: %v", err)

		return err
	}
	err = cancelJob(context.Background(), client, k8sJobName(job.JID), jke.rt.config.Namespace)
	if err != nil {
		log.Printf("[executor] failed to cancel the Job: %v", err)
	}

	return err
}

// Name is "kubernetes".
func (jke JKubernetesExecutor) Name() string { return "kubernetes" }

// Status checks the cluster the executor would use.
func (jke JKubernetesExecutor) Status(ctx context.Context) model.ExecutorStatus {
	ns := jke.rt.config.Namespace
	client, err := k.GetKubeClient()
	if err != nil {
		return model.ExecutorStatus{Engine: "kubernetes", Where: "namespace " + ns, CheckedAt: ut.CurrentTime(),
			Error: "no cluster configuration: " + err.Error()}
	}

	return kubeStatus(ctx, client, ns)
}

// kubeStatus asks the API server for its version and lists jobs in the
// namespace (what the executor needs permission for).
func kubeStatus(ctx context.Context, client kubernetes.Interface, ns string) model.ExecutorStatus {
	st := model.ExecutorStatus{Engine: "kubernetes", Where: "namespace " + ns, CheckedAt: ut.CurrentTime()}
	ctx, cancel := context.WithTimeout(ctx, executorStatusTimeout)
	defer cancel()
	// ServerVersion takes no context: bound it here
	type answer struct {
		version string
		err     error
	}
	done := make(chan answer, 1)
	go func() {
		v, err := client.Discovery().ServerVersion()
		if err != nil {
			done <- answer{err: err}

			return
		}
		done <- answer{version: v.GitVersion}
	}()
	select {
	case a := <-done:
		if a.err != nil {
			st.Error = "API server: " + a.err.Error()

			return st
		}
		st.Version = a.version
	case <-ctx.Done():
		st.Error = "API server: no answer within " + executorStatusTimeout.String()

		return st
	}
	if _, err := client.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
		st.Error = "namespace " + ns + ": " + err.Error()

		return st
	}
	st.Active = true

	return st
}

// jobQuotas are the parsed resource requests/limits of a job.
type jobQuotas struct {
	reqMem, reqCPU, limMem, limCPU resource.Quantity
}

// parseJobQuotas validates the job's resource strings. resource.MustParse on
// user input used to panic - and a panic in a worker goroutine kills uspace.
func parseJobQuotas(job model.Job) (jobQuotas, error) {
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

func cancelJob(ctx context.Context, clientset kubernetes.Interface, jobName, namespace string) error {
	foreground := metav1.DeletePropagationForeground

	return clientset.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{
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
	go streamToSocketWS(jobsSocketAddress, jid, o.ch) // the address is read here, not in the stream

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

func executeK8sJob(je *JKubernetesExecutor, job model.Job) {
	jobName := k8sJobName(job.JID)
	namespace := je.rt.config.Namespace
	out := newJobOutput(job.JID)
	defer func() {
		// the log outlives the live stream: save it with the job
		if err := je.rt.saveJobLog(context.Background(), job.JID, out.Close()); err != nil {
			log.Printf("[executor] failed to save the log of job %d: %v", job.JID, err)
		}
	}()

	out.Send([]byte("=-----------------------------------------------------------------------="))
	out.Sendf("[executor] formatting job as %s\n", jobName)

	command, err := je.rt.prepareJobRun(&job)
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
		int64(job.Timeout*60), je.rt.config.UspaceJobTTL)

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
	// a user's cancel (jobRuntime.cancel) cancels this with ErrJobCancelled
	ctx, cancel := context.WithCancelCause(deadline)
	defer cancel(nil)
	untrack := je.rt.trackRunning(job.JID, cancel)
	defer untrack()
	if je.rt.takeCancelled(job.JID) { // cancelled between dequeue and here
		cancel(ErrJobCancelled)
	}

	if err := runJob(ctx, clientset, jobSpec, namespace); err != nil {
		log.Printf("error starting job: %v", err)
		out.Sendf("[executor]: error launching job execution %v\n", err)
		je.markFailed(job.JID, 0)

		return
	}
	startTime := time.Now()
	if err := je.rt.markJobStatus(context.Background(), job.JID, "running", 0); err != nil {
		log.Printf("failed to mark job %d running: %v", job.JID, err)
	}
	if err := je.rt.markJobEngine(context.Background(), job.JID, "kubernetes"); err != nil {
		log.Printf("failed to record job %d engine: %v", job.JID, err)
	}
	je.watch(ctx, clientset, job, jobSpec.Name, namespace, out, startTime)
}

// watch follows a started kubernetes job to its end: streams its logs,
// waits for its status (deleting it if its owner cancelled ctx), then
// records the outcome and, when it completed, its output. ExecuteJob and
// Reattach share it.
func (jke JKubernetesExecutor) watch(ctx context.Context, clientset kubernetes.Interface, job model.Job,
	jobName, namespace string, out *jobOutput, startTime time.Time,
) {
	// the outcome is written even when ctx was cancelled
	bg := context.WithoutCancel(ctx)
	var logsDone sync.WaitGroup
	logsDone.Add(1)
	go func() {
		defer logsDone.Done()
		if err := streamJobLogs(ctx, clientset, jobName, namespace, out.Send); err != nil {
			log.Printf("failed to stream job logs: %v", err)
			out.Sendf("[executor]: error streaming pod logs: %v\n", err)
		}
	}()

	status, err := monitorJob(ctx, clientset, jobName, namespace)
	if err != nil {
		log.Printf("error monitoring job: %v", err)
	}
	if errors.Is(context.Cause(ctx), ErrJobCancelled) {
		// stop the pod: deleting the kubernetes job (foreground) removes it
		if err := cancelJob(bg, clientset, jobName, namespace); err != nil {
			log.Printf("failed to delete cancelled job %s: %v", jobName, err)
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

	if err := jke.rt.markJobStatus(bg, job.JID, status, duration); err != nil {
		log.Printf("failed to annotate result to database")
		out.Sendf("[executor]: error marking job completion %v\n", err)
	}

	if status == "completed" {
		jke.rt.recordJobOutput(bg, job, out)
	}
}

func (jke JKubernetesExecutor) markFailed(jid int64, d time.Duration) {
	if err := jke.rt.markJobStatus(context.Background(), jid, "failed", d); err != nil {
		log.Printf("failed to mark job %d failed: %v", jid, err)
	}
}

// prepareJobRun fills in a job's defaults, image, command and environment
// (including its presigned input/output URLs). Shared by the executors.
func (rt *jobRuntime) prepareJobRun(job *model.Job) ([]string, error) {
	// handle some generic checks as guard statement
	if !ut.AssertStructNotEmptyUpon(job, map[any]bool{
		"Input":     true,
		"Output":    true,
		"Logic":     true,
		"LogicBody": true,
	}) {
		return nil, fmt.Errorf("the job has no input or output (%w)", ut.ErrInvalid)
	}

	// Assuming job.Logic is the image name and job.LogicBody is the command
	var (
		InpAsResource model.Resource
		OutAsResource model.Resource
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
		InpAsResource.Vname = rt.storage.DefaultVolume()
		InpAsResource.Name = job.Input
	}

	parts = strings.Split(job.Output, "/")
	if len(parts) > 1 {
		OutAsResource.Vname = parts[0]
		OutAsResource.Name = strings.Join(parts[1:], "/")
	} else {
		OutAsResource.Vname = rt.storage.DefaultVolume()
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
	inputURL, outputURL, err := presignJobIO(rt.storage, InpAsResource, OutAsResource, jobURLValidity(job.Timeout))
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

// jobURLValidity is how long a job's input/output URLs stay valid: its
// timeout plus slack for scheduling, or 6 hours for jobs without one.
func jobURLValidity(timeoutMinutes int) time.Duration {
	if timeoutMinutes > 0 {
		return time.Duration(timeoutMinutes)*time.Minute + 15*time.Minute
	}

	return 6 * time.Hour
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

func formatJobCommand(job *model.Job) ([]string, error) {
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

// OutputGID is the group a job's outputs belong to: the owner's primary group
// recorded at submission (older jobs: the owner's uid).
func OutputGID(job model.Job) int64 {
	if job.GID > 0 {
		return job.GID
	}

	return job.UID
}

// presignJobIO asks the storage for the job's input and output URLs.
func presignJobIO(storage Storage, in, out model.Resource, valid time.Duration) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return storage.PresignJobIO(ctx, in, out, valid)
}

// recordJobOutput records a completed job's output as its owner's file
// (the storage's RecordOutput), reporting in the job's log. It runs after
// the job, so ctx is not a request context.
func (rt *jobRuntime) recordJobOutput(ctx context.Context, job model.Job, out *jobOutput) {
	out.Sendf("[executor] saving output %s ...\n", job.Output)
	action, err := rt.storage.RecordOutput(ctx, job)
	if err != nil {
		log.Printf("failed to record output of job %d: %v", job.JID, err)
		out.Sendf("[executor]: error recording the output: %v\n", err)

		return
	}
	out.Sendf("[executor] output %s\n", action)
	out.Send([]byte("[executor] OK.\n"))
}
