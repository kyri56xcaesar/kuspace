package uspace

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	k "kyri56xcaesar/kuspace/internal/uspace/kubernetes"
	ut "kyri56xcaesar/kuspace/internal/utils"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type JKubernetesExecutor struct {
	jm *JobManager
}

func NewJKubernetesExecutor(jm *JobManager) JKubernetesExecutor {
	return JKubernetesExecutor{
		jm: jm,
	}
}

func (jke JKubernetesExecutor) ExecuteJob(job ut.Job) error {
	defer func() { <-jke.jm.workerPool }() // release worker slot
	executeK8sJob(&jke, job)
	return nil
}
func (jke JKubernetesExecutor) CancelJob(job ut.Job) error {
	cancelJob(k.GetKubeClient(), fmt.Sprintf("job-%d", job.Jid), "default")
	return nil

}

func buildK8sJob(
	jobID string,
	image string,
	command []string,
	env map[string]string,
	quotas map[string]string,
	parallelism int32,
) *batchv1.Job {
	envVars := []corev1.EnvVar{}
	for k, v := range env {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("job-%s", jobID),
			Labels: map[string]string{"job-group": "uspace-job"},
		},
		Spec: batchv1.JobSpec{
			Parallelism:  &parallelism,
			Completions:  &parallelism,
			BackoffLimit: pointerToInt32(0),
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
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil || len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for job: %v", err)
	}

	podName := pods.Items[0].Name
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow: true,
	})

	stream, err := req.Stream(context.TODO())
	if err != nil {
		return err
	}
	defer stream.Close()

	buf := make([]byte, 2000)
	for {
		n, err := stream.Read(buf)
		if err != nil {
			break
		}
		send(buf[:n]) // You can push this to your WebSocket, etc.
	}
	return nil
}

func executeK8sJob(je *JKubernetesExecutor, job ut.Job) {
	jobName := fmt.Sprintf("j-%d", job.Jid)

	err := formatJobData(je, &job)
	if err != nil {
		log.Printf("error formatting job data: %v", err)
		return
	}

	command, err := formatJobCommand(job.Logic, job.LogicBody)
	if err != nil {
		log.Printf("error formatting job command: %v", err)
		return
	}

	jobSpec := buildK8sJob(
		jobName,
		job.Logic,
		command,
		job.Env,
		map[string]string{"RMem": job.MemoryRequest, "RCpu": job.CpuRequest, "LMem": job.MemoryLimit, "LCpu": job.CpuLimit},
		int32(job.Parallelism), // parallelism // should default to 1
	)

	clientset := k.GetKubeClient() // from your config
	err = runJob(clientset, jobSpec, "default")
	if err != nil {
		log.Printf("error starting job: %v", err)
		return
	}

	go streamJobLogs(clientset, jobSpec.Name, "default", func(data []byte) {
		streamToSocketWS(job.Jid, bytes.NewReader(data))
	})

	status, err := monitorJob(clientset, jobSpec.Name, "default")
	if err != nil {
		log.Printf("error monitoring job: %v", err)
	}

	log.Printf("Job %v finished with status: %s", jobName, status)

	// Optional: cleanup or postprocess
}

func formatJobData(je *JKubernetesExecutor, job *ut.Job) error {
	// handle some generic checks as guard statement
	if !ut.AssertStructNotEmptyUpon(job, map[any]bool{
		"Input":     true,
		"Output":    true,
		"Logic":     true,
		"LogicBody": true,
	}) {
		return ut.NewError("empty field that shouldn't be empty..")
	}

	// Assuming job.Logic is the image name and job.LogicBody is the command
	var (
		name, version string
		InpAsResource ut.Resource
		OutAsResource ut.Resource
	)

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
		return fmt.Errorf("invalid job data")
	}
	job.Logic = fmt.Sprintf("%s:%s", name, version)

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
	if job.OutputFormat == "" { //default format
		job.OutputFormat = "txt"
	}
	if job.InputFormat == "" {
		job.InputFormat = "txt"
	}
	if job.Parallelism == 0 { //default parallelism
		job.Parallelism = 1
	}

	if job.MemoryLimit == "" {
		job.MemoryLimit = "4Gi" // default limit
	}

	if job.CpuLimit == "" {
		job.CpuLimit = "1000m"
	}

	if job.MemoryRequest == "" {
		job.MemoryRequest = "2Gi"
	}

	if job.CpuRequest == "" {
		job.CpuRequest = "500m"
	}

	envMap["ENDPOINT"] = je.jm.srv.config.MINIO_ENDPOINT
	envMap["ACCESS_KEY"] = je.jm.srv.config.MINIO_ACCESS_KEY
	envMap["SECRET_KEY"] = je.jm.srv.config.MINIO_SECRET_KEY
	envMap["LOGIC"] = job.LogicBody
	envMap["INPUT_BUCKET"] = InpAsResource.Vname
	envMap["INPUT_OBJECT"] = InpAsResource.Name
	envMap["INPUT_FORMAT"] = job.InputFormat
	envMap["OUTPUT_BUCKET"] = OutAsResource.Vname
	envMap["OUTPUT_OBJECT"] = OutAsResource.Name
	envMap["OUTPUT_FORMAT"] = job.OutputFormat
	envMap["TIMEOUT"] = fmt.Sprintf("%d", job.Timeout)
	job.Env = envMap

	return nil
}

func formatJobCommand(logic, body string) ([]string, error) {
	lang := logic[:strings.Index(logic+":", ":")]
	switch lang {
	case "python", "py":
		return []string{"/bin/sh", "-c", fmt.Sprintf("python3 -c '%s'", body)}, nil
	case "bash", "sh", "shell":
		return []string{"/bin/sh", "-c", fmt.Sprintf("bash -c '%s'", body)}, nil
	case "go", "golang":
		return []string{"/bin/sh", "-c", fmt.Sprintf("echo '%s' > /tmp/tmp.go && go run /tmp/tmp.go && rm /tmp/tmp.go", body)}, nil
	case "java", "javac", "openjdk":
		return []string{"/bin/sh", "-c", fmt.Sprintf(`cat <<EOF > /tmp/Tmp.java
		%s
		EOF
		javac /tmp/Tmp.java && java -cp /tmp Tmp && rm /tmp/Tmp.java /tmp/Tmp.class`, body)}, nil
	case "node", "javascript", "js":
		return []string{"/bin/sh", "-c", fmt.Sprintf("node -e '%s'", body)}, nil

	case "application/duckdb": // check if the given logic is a custom app
		return []string{"python", "duckdb_app.py"}, nil

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
