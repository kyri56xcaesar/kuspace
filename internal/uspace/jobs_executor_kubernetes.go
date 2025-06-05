package uspace

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
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

// CancelJob method responsible for canceling the job execution
func (jke JKubernetesExecutor) CancelJob(job ut.Job) error {
	client, err := k.GetKubeClient()
	if err != nil {
		log.Printf("[executor] could not retrieve k8s client: %v", err)
		return err
	}
	err = cancelJob(client, fmt.Sprintf("job-%d", job.Jid), jke.jm.srv.config.Namespace)
	if err != nil {
		log.Printf("[executor] failed to cancel the Job: %v", err)
	}
	return err

}

func buildK8sJob(
	jobID string,
	image string,
	command []string,
	env map[string]string,
	quotas map[string]string,
	parallelism int32,
	namespace string,
	timeout int64,
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
			Name:      fmt.Sprintf("job-%s", jobID),
			Labels:    map[string]string{"job-group": "uspace-job", "job-name": jobID},
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds: deadlinePtr,
			Parallelism:           &parallelism,
			Completions:           &parallelism,
			BackoffLimit:          pointerToInt32(0),
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
								corev1.ResourceMemory: resource.MustParse(quotas["RMem"]),
								corev1.ResourceCPU:    resource.MustParse(quotas["RCpu"]),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse(quotas["LMem"]),
								corev1.ResourceCPU:    resource.MustParse(quotas["LCpu"]),
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

func runJob(clientset *kubernetes.Clientset, job *batchv1.Job, namespace string) error {
	_, err := clientset.BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	return err
}

func cancelJob(clientset *kubernetes.Clientset, jobName, namespace string) error {
	foreground := metav1.DeletePropagationForeground
	return clientset.BatchV1().Jobs(namespace).Delete(context.TODO(), jobName, metav1.DeleteOptions{
		PropagationPolicy: &foreground,
	})
}

func monitorJob(clientset *kubernetes.Clientset, jobName, namespace string) (string, error) {
	watcher, err := clientset.BatchV1().Jobs(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", jobName),
	})
	if err != nil {
		return "", err
	}

	for event := range watcher.ResultChan() {
		j := event.Object.(*batchv1.Job)
		if j.Status.Succeeded > 0 {
			return "completed", nil
		}
		if j.Status.Failed > 0 {
			return "failed", nil
		}
	}
	return "unknown", fmt.Errorf("watch ended unexpectedly")
}

func streamJobLogs(clientset *kubernetes.Clientset, jobName, namespace string, send func([]byte)) error {
	labelSelector := fmt.Sprintf("job-name=job-%s", jobName)

	var podName string
	timeout := time.After(60 * time.Second)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	// Step 1: Wait for pod to appear
WAIT_FOR_CREATION:
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for pod creation for job: %s", jobName)
		case <-tick.C:
			pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				return fmt.Errorf("error listing pods: %v", err)
			}
			if len(pods.Items) > 0 {
				podName = pods.Items[0].Name
				// log.Printf("found pod: %s for job: %s", podName, jobName)
				break WAIT_FOR_CREATION
			}
		}
	}

	// Step 2: Wait for pod readiness
	timeout = time.After(60 * time.Second)
WAIT_FOR_READY:
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for pod %s to be ready", podName)
		case <-tick.C:
			pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get pod status: %v", err)
			}
			if pod.Status.Phase == corev1.PodRunning {
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.Ready {
						// log.Printf("pod %s is ready", podName)
						break WAIT_FOR_READY
					}
				}
			}
		}
	}

	// Step 3: Start streaming logs
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow: true,
	})

	stream, err := req.Stream(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to open log stream for pod %s: %w", podName, err)
	}
	defer func() {
		err := stream.Close()
		if err != nil {
			log.Printf("failed to close the stream: %v", err)
		}
	}()

	reader := bufio.NewReader(stream)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading log stream: %w", err)
		}
		send(append([]byte("\t[POD]"), line...))
	}
	return nil
}

func executeK8sJob(je *JKubernetesExecutor, job ut.Job) {
	jobName := fmt.Sprintf("j-%d", job.Jid)
	namespace := je.jm.srv.config.Namespace
	// llets create a stream channel (for the websocket)
	wsChan := make(chan []byte, 100)
	// begin streaming channel, also set a deadline,  perhaps a context with a deadline could work
	go streamToSocketWS(job.Jid, wsChan)
	go func() {
		time.Sleep(time.Second * 500)
		close(wsChan) // this propably lasts longer than the prev context

	}()

	wsChan <- []byte("...")
	wsChan <- []byte("=-----------------------------------------------------------------------=")
	wsChan <- []byte("...")
	wsChan <- []byte(fmt.Sprintf("[executor] formatting Job as job-name: job-%s\n", jobName))

	command, err := formatJobData(je, &job)
	if err != nil {
		log.Printf("error formatting job data: %v", err)
		wsChan <- []byte(fmt.Sprintf("[executor]: error formatting job data %v\n", err))

		return
	}

	wsChan <- []byte("[executor] constructing job...\n")

	jobSpec := buildK8sJob(
		jobName,
		job.Logic,
		command,
		job.Env,
		map[string]string{"RMem": job.MemoryRequest, "RCpu": job.CPURequest, "LMem": job.MemoryLimit, "LCpu": job.CPULimit},
		int32(job.Parallelism), // parallelism // should default to 1
		namespace,
		int64(job.Timeout),
	)

	wsChan <- []byte("[executor] launcing job...\n")
	wsChan <- []byte(fmt.Sprintf("[executor] specs: {parallelism: %v, timeout: %v, cpu_limit: %v, cpu_request: %v, mem_limit: %v, mem_req: %v, storage_limit: %v, storage_request: %v}\n", job.Parallelism, job.Timeout, job.CPULimit, job.CPURequest, job.MemoryLimit, job.MemoryRequest, job.EphimeralStorageLimit, job.EphimeralStorageRequest))

	clientset, err := k.GetKubeClient() // from config
	if err != nil {
		log.Printf("[executor] could not retrieve kube client: %v", err)
		wsChan <- []byte("could not retrieve k8s client, fatal...\nexiting...")
		return
	}
	err = runJob(clientset, jobSpec, namespace)
	if err != nil {
		log.Printf("error starting job: %v", err)
		wsChan <- []byte(fmt.Sprintf("[executor]: error launching job execution%v\n", err))
		return
	}
	startTime := time.Now()

	// monitor and stream the logs of that job
	go func() {
		err = streamJobLogs(clientset, jobName, namespace, func(data []byte) {
			wsChan <- data
		})
		if err != nil {
			log.Printf("failed to stream job logs.. :%v", err)
			wsChan <- []byte(fmt.Sprintf("[executor]: error streaming pod logs: %v\n", err))
		}
	}()

	status, err := monitorJob(clientset, jobSpec.Name, namespace)
	if err != nil {
		log.Printf("error monitoring job: %v", err)
	}
	duration := time.Since(startTime)
	// log.Printf("[executor] Job %v finished with status: %s, duration: %v", jobName, status, duration)
	wsChan <- []byte(fmt.Sprintf("[executor] Job %v finished with status: %s, duration: %v\n", jobName, status, duration))

	// Optional: cleanup or postprocess
	err = je.jm.srv.markJobStatus(job.Jid, status, duration)
	if err != nil {
		log.Printf("failed to annotate result to database")
		wsChan <- []byte(fmt.Sprintf("[executor]: error marking job completion%v\n", err))
	}

	// save output to db
	p := strings.SplitN(job.Output, "/", 2)
	if len(p) != 2 {
		log.Printf("job output was invalid format, should have escaped by now...")
	}

	if status == "completed" {
		outputResource := ut.Resource{
			Name:  p[1],
			Path:  "/",
			Type:  "file",
			Perms: "rw-r--r--",
			UID:   job.UID,
			Vname: p[0],
			Gid:   job.UID,
		}
		info, err := je.jm.srv.storage.Stat(outputResource)
		if err != nil {
			log.Printf("failed to stat output file from storage: %v", err)
			wsChan <- []byte(fmt.Sprintf("[executor]: error retrieving output file %v\n", err))
			return
		}
		infoCasted, ok := info.(minio.ObjectInfo) // this should be changed to be independent of minio... // will do "resourceInfo struct "
		if !ok {
			log.Printf("failed to cast to object info")
			wsChan <- []byte(fmt.Sprintf("[executor]: error retrieving output file format %v\n", err))
			return
		}
		outputResource.Size = infoCasted.Size

		// log.Printf("[executor]...saving output in database...")
		wsChan <- []byte(fmt.Sprintf("[executor] saving output %s/%s ...\n", outputResource.Vname, outputResource.Name))
		_, err = je.jm.srv.fsl.Insert(outputResource)
		if err != nil {
			log.Printf("failed to insert output object in database: %v", err)
			wsChan <- []byte(fmt.Sprintf("[executor]: error saving output data in db...%v\n", err))
		}

		wsChan <- []byte("[executor] OK.\n")
	}
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
			job.OutputFormat = "txt" //default format
		} else {
			job.OutputFormat = p[len(p)-1]
		}
	}
	if job.InputFormat == "" {
		// deduce input format
		p := strings.Split(job.Input, ".")
		if len(p) == 0 || p[len(p)-1] == "" {
			job.InputFormat = "txt" //default format
		} else {
			job.InputFormat = p[len(p)-1]
		}
	}
	if job.Parallelism == 0 { //default parallelism
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
	envMap["TIMEOUT"] = fmt.Sprintf("%d", job.Timeout)
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
		return nil, fmt.Errorf("invalid job data")
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
		return []string{"/bin/sh", "-c", fmt.Sprintf("echo '%s' > /tmp/tmp.go && go run /tmp/tmp.go && rm /tmp/tmp.go", body)}, nil
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
		return []string{"/bin/sh", "-c", fmt.Sprintf("cat <<EOF > /tmp/tmp.c \n%s\nEOF && gcc /tmp/tmp.c -o /tmp/tmp.out && /tmp/tmp.out && rm /tmp/tmp.*", body)}, nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}
