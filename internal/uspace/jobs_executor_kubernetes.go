package uspace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
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
	duckImage     = "kyri56xcaesar/kuspace:applications-duckdb-v1"
	pandasImage   = "kyri56xcaesar/kuspace:applications-pypandas-v1"
	octaveImage   = "kyri56xcaesar/kuspace:applications-octave-v1"
	ffmpegImage   = "kyri56xcaesar/kuspace:applications-ffmpeg-v1"
	caengineImage = "kyri56xcaesar/kuspace:applications-caengine-v1"
	bashImage     = "kyri56xcaesar/kuspace:applications-bash-v1"
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

	command, err := formatJobData(je, &job)
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
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()

	if err := runJob(ctx, clientset, jobSpec, namespace); err != nil {
		log.Printf("error starting job: %v", err)
		out.Sendf("[executor]: error launching job execution %v\n", err)
		je.markFailed(job.JID, 0)

		return
	}
	startTime := time.Now()

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
		je.recordOutput(job, out)
	}
}

func (je *JKubernetesExecutor) markFailed(jid int64, d time.Duration) {
	if err := je.jm.srv.markJobStatus(context.Background(), jid, "failed", d); err != nil {
		log.Printf("failed to mark job %d failed: %v", jid, err)
	}
}

// recordOutput adds the job's output object to the metadata store and charges
// it to the owner's quota (unenforced: the object already exists).
func (je *JKubernetesExecutor) recordOutput(job ut.Job, out *jobOutput) {
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
		VID:   je.jm.srv.volumeID(vname),
		GID:   job.UID,
	}
	info, err := je.jm.srv.storage.Stat(outputResource)
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
	if _, err := je.jm.srv.fsl.Insert(outputResource); err != nil {
		log.Printf("failed to insert output object in database: %v", err)
		out.Sendf("[executor]: error saving output data in db... %v\n", err)

		return
	}
	quota := min(je.jm.srv.config.LocalVolumesDefaultCapacity, maxDefaultVolumeCapacity)
	if err := je.jm.srv.fsl.ClaimSpace(context.Background(), job.UID, vname, outputResource.Size, quota, false); err != nil {
		log.Printf("failed to account output of job %d: %v", job.JID, err)
	}
	out.Send([]byte("[executor] OK.\n"))
}

func formatJobData(je *JKubernetesExecutor, job *ut.Job) ([]string, error) {
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
		InpAsResource.Vname = je.jm.srv.storage.DefaultVolume(false)
		InpAsResource.Name = job.Input
	}

	parts = strings.Split(job.Output, "/")
	if len(parts) > 1 {
		OutAsResource.Vname = parts[0]
		OutAsResource.Name = strings.Join(parts[1:], "/")
	} else {
		OutAsResource.Vname = je.jm.srv.storage.DefaultVolume(false)
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

	envMap["ENDPOINT"] = je.jm.srv.config.MinioEndpoint
	envMap["ACCESS_KEY"] = je.jm.srv.config.MinioAccessKey
	envMap["SECRET_KEY"] = je.jm.srv.config.MinioSecretKey
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

func formatJobCommand(job *ut.Job) ([]string, error) {
	var name, version string
	// deduct name and version and format it
	p := strings.Split(strings.TrimSpace(job.Logic), ":")
	if len(p) == 2 {
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
	body := job.LogicBody

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
	case "python", "py":

		return []string{"/bin/sh", "-c", fmt.Sprintf("python3 -c '%s'", body)}, nil
	case "go", "golang":

		return []string{"/bin/sh", "-c",
			fmt.Sprintf("echo '%s' > /tmp/tmp.go && go run /tmp/tmp.go && rm /tmp/tmp.go", body)}, nil
	case "java", "javac", "openjdk":

		return []string{"/bin/sh", "-c", fmt.Sprintf(`cat <<EOF > /tmp/Tmp.java
		%s
		EOF
		javac /tmp/Tmp.java && java -cp /tmp Tmp && rm /tmp/Tmp.java /tmp/Tmp.class`, body)}, nil
	case "node", "javascript", "js":

		return []string{"/bin/sh", "-c", fmt.Sprintf("node -e '%s'", body)}, nil

	case "ruby":

		return []string{"/bin/sh", "-c", fmt.Sprintf("ruby -e '%s'", body)}, nil
	case "php":

		return []string{"/bin/sh", "-c", fmt.Sprintf("php -r '%s'", body)}, nil
	case "perl":

		return []string{"/bin/sh", "-c", fmt.Sprintf("perl -e '%s'", body)}, nil
	case "rust":

		return []string{"/bin/sh", "-c", fmt.Sprintf("rustc -e '%s'", body)}, nil
	case "swift":

		return []string{"/bin/sh", "-c", fmt.Sprintf("swift -e '%s'", body)}, nil
	case "typescript":

		return []string{"/bin/sh", "-c", fmt.Sprintf("ts-node -e '%s'", body)}, nil
	case "scala":

		return []string{"/bin/sh", "-c", fmt.Sprintf("scala -e '%s'", body)}, nil
	case "haskell":

		return []string{"/bin/sh", "-c", fmt.Sprintf("runhaskell -e '%s'", body)}, nil
	case "kotlin":

		return []string{"/bin/sh", "-c", fmt.Sprintf("kotlin -e '%s'", body)}, nil
	case "elixir":

		return []string{"/bin/sh", "-c", fmt.Sprintf("elixir -e '%s'", body)}, nil
	case "lua":

		return []string{"/bin/sh", "-c", fmt.Sprintf("lua -e '%s'", body)}, nil
	case "r":

		return []string{"/bin/sh", "-c", fmt.Sprintf("Rscript -e '%s'", body)}, nil
	case "dart":

		return []string{"/bin/sh", "-c", fmt.Sprintf("dart -e '%s'", body)}, nil
	case "powershell":

		return []string{"/bin/sh", "-c", fmt.Sprintf("pwsh -c '%s'", body)}, nil
	case "sql":

		return []string{"/bin/sh", "-c", fmt.Sprintf("sqlcmd -Q '%s'", body)}, nil
	case "groovy":

		return []string{"/bin/sh", "-c", fmt.Sprintf("groovy -e '%s'", body)}, nil
	case "clojure":

		return []string{"/bin/sh", "-c", fmt.Sprintf("clojure -e '%s'", body)}, nil
	case "objective-c":

		return []string{"/bin/sh", "-c", fmt.Sprintf("clang -x objective-c -e '%s'", body)}, nil
	case "visual-basic":

		return []string{"/bin/sh", "-c", fmt.Sprintf("vbc -e '%s'", body)}, nil
	case "assembly":

		return []string{"/bin/sh", "-c", fmt.Sprintf("nasm -e '%s'", body)}, nil
	case "fortran":

		return []string{"/bin/sh", "-c", fmt.Sprintf("gfortran -e '%s'", body)}, nil
	case "pascal":

		return []string{"/bin/sh", "-c", fmt.Sprintf("fpc -e '%s'", body)}, nil
	case "prolog":

		return []string{"/bin/sh", "-c", fmt.Sprintf("swipl -e '%s'", body)}, nil
	case "scheme":

		return []string{"/bin/sh", "-c", fmt.Sprintf("guile -c '%s'", body)}, nil
	case "tcl":

		return []string{"/bin/sh", "-c", fmt.Sprintf("tclsh -e '%s'", body)}, nil
	case "smalltalk":

		return []string{"/bin/sh", "-c", fmt.Sprintf("gst -e '%s'", body)}, nil
	case "nim":

		return []string{"/bin/sh", "-c", fmt.Sprintf("nim c -d:nodebug -e '%s'", body)}, nil
	case "ocaml":

		return []string{"/bin/sh", "-c", fmt.Sprintf("ocaml -e '%s'", body)}, nil
	case "f#":

		return []string{"/bin/sh", "-c", fmt.Sprintf("fsharpi -e '%s'", body)}, nil
	case "crystal":

		return []string{"/bin/sh", "-c", fmt.Sprintf("crystal eval '%s'", body)}, nil
	case "reason":

		return []string{"/bin/sh", "-c", fmt.Sprintf("reason-cli -e '%s'", body)}, nil
	case "d":

		return []string{"/bin/sh", "-c", fmt.Sprintf("dmd -run '%s'", body)}, nil
	case "solidity":

		return []string{"/bin/sh", "-c", fmt.Sprintf("solc --bin '%s'", body)}, nil
	case "v":

		return []string{"/bin/sh", "-c", fmt.Sprintf("v run '%s'", body)}, nil
	case "zig":

		return []string{"/bin/sh", "-c", fmt.Sprintf("zig run '%s'", body)}, nil
	case "vala":

		return []string{"/bin/sh", "-c", fmt.Sprintf("valac --pkg gtk+-3.0 '%s'", body)}, nil
	case "c", "gcc":

		return []string{"/bin/sh", "-c",
			fmt.Sprintf("cat <<EOF > /tmp/tmp.c \n%s\nEOF && gcc /tmp/tmp.c -o /tmp/tmp.out && /tmp/tmp.out && rm /tmp/tmp.*",
				body)}, nil
	default:

		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}
