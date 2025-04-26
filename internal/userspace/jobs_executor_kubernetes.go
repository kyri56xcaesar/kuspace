package userspace

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	k "kyri56xcaesar/myThesis/internal/userspace/kubernetes"
	ut "kyri56xcaesar/myThesis/internal/utils"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type JKubernetesExecutor struct {
}

func NewJKubernetesExecutor() JKubernetesExecutor {
	return JKubernetesExecutor{}
}

func (jke JKubernetesExecutor) ExecuteJob(job ut.Job) error {
	executeK8sJob(job)
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
	parallelism int32,
) *batchv1.Job {
	envVars := []corev1.EnvVar{}
	for k, v := range env {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("job-%s", jobID),
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

func formatJobData(job *ut.Job) error {
	// handle some generic checks as guard statement
	if job.Input == nil {
		return fmt.Errorf("input is empty")
	}
	if job.Output == "" {
		return fmt.Errorf("output is empty")
	}
	if job.Logic == "" {
		return fmt.Errorf("logic is empty")
	}
	if job.Parallelism == 0 {
		job.Parallelism = 1
	}
	// Assuming job.Logic is the image name and job.LogicBody is the command
	var (
		name, version string
	)

	// deduct name and version
	p := strings.Split(job.Logic, ":")
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

	// check if the given logic is a custom app
	if strings.Contains(name, "application") {
		// handle this case (later)
		appName := strings.TrimPrefix(name, "application/")
		log.Print(appName)

		// what do we need to do here ?
		return nil
	} else { // this is a code job
		// format the logic
		if job.LogicBody == "" {
			return fmt.Errorf("logic body is empty")
		}

		job.Logic = fmt.Sprintf("%s:%s", name, version)
		body, err := formatJobBody(name, job.LogicBody)
		if err != nil {
			return fmt.Errorf("error formatting job body: %v", err)
		}
		job.LogicBody = body

	}

	// format i/o variables
	// format and setup environment variables
	job.Output = strings.TrimSpace(job.Output)
	job.OutputFormat = strings.TrimSpace(job.OutputFormat)
	if job.OutputFormat == "" {
		job.OutputFormat = "csv"
	}
	job.Env = map[string]string{
		"INPUT":         strings.Join(job.Input, ","),
		"OUTPUT":        job.Output,
		"OUTPUT_FORMAT": job.OutputFormat,
		"LOGIC":         job.LogicBody,
	}

	log.Printf("job formatted to :%+v", job)

	return nil
}

func executeK8sJob(job ut.Job) {
	jobName := fmt.Sprintf("j-%d", job.Jid)

	err := formatJobData(&job)
	if err != nil {
		log.Printf("error formatting job data: %v", err)
		return
	}

	jobSpec := buildK8sJob(
		jobName,
		job.Logic,
		[]string{"/bin/sh", "-c", job.LogicBody},
		job.Env,
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

func formatJobBody(lng, body string) (string, error) {
	switch lng {
	case "python", "py":
		return fmt.Sprintf("python3 -c '%s'", body), nil
	case "bash", "sh", "shell":
		return fmt.Sprintf("bash -c '%s'", body), nil
	case "go", "golang":
		return fmt.Sprintf("echo '%s' > /tmp/tmp.go && go run /tmp/tmp.go && rm /tmp/tmp.go", body), nil
	case "java", "javac", "openjdk":
		return fmt.Sprintf(`cat <<EOF > /tmp/Tmp.java
		%s
		EOF
		javac /tmp/Tmp.java && java -cp /tmp Tmp && rm /tmp/Tmp.java /tmp/Tmp.class`, body), nil
	case "node", "javascript", "js":
		return fmt.Sprintf("node -e '%s'", body), nil
	case "ruby":
		return fmt.Sprintf("ruby -e '%s'", body), nil
	case "php":
		return fmt.Sprintf("php -r '%s'", body), nil
	case "perl":
		return fmt.Sprintf("perl -e '%s'", body), nil
	case "rust":
		return fmt.Sprintf("rustc -e '%s'", body), nil
	case "swift":
		return fmt.Sprintf("swift -e '%s'", body), nil
	case "typescript":
		return fmt.Sprintf("ts-node -e '%s'", body), nil
	case "scala":
		return fmt.Sprintf("scala -e '%s'", body), nil
	case "haskell":
		return fmt.Sprintf("runhaskell -e '%s'", body), nil
	case "kotlin":
		return fmt.Sprintf("kotlin -e '%s'", body), nil
	case "elixir":
		return fmt.Sprintf("elixir -e '%s'", body), nil
	case "lua":
		return fmt.Sprintf("lua -e '%s'", body), nil
	case "r":
		return fmt.Sprintf("Rscript -e '%s'", body), nil
	case "dart":
		return fmt.Sprintf("dart -e '%s'", body), nil
	case "powershell":
		return fmt.Sprintf("pwsh -c '%s'", body), nil
	case "sql":
		return fmt.Sprintf("sqlcmd -Q '%s'", body), nil
	case "groovy":
		return fmt.Sprintf("groovy -e '%s'", body), nil
	case "clojure":
		return fmt.Sprintf("clojure -e '%s'", body), nil
	case "objective-c":
		return fmt.Sprintf("clang -x objective-c -e '%s'", body), nil
	case "visual-basic":
		return fmt.Sprintf("vbc -e '%s'", body), nil
	case "assembly":
		return fmt.Sprintf("nasm -e '%s'", body), nil
	case "fortran":
		return fmt.Sprintf("gfortran -e '%s'", body), nil
	case "pascal":
		return fmt.Sprintf("fpc -e '%s'", body), nil
	case "prolog":
		return fmt.Sprintf("swipl -e '%s'", body), nil
	case "scheme":
		return fmt.Sprintf("guile -c '%s'", body), nil
	case "tcl":
		return fmt.Sprintf("tclsh -e '%s'", body), nil
	case "smalltalk":
		return fmt.Sprintf("gst -e '%s'", body), nil
	case "nim":
		return fmt.Sprintf("nim c -d:nodebug -e '%s'", body), nil
	case "ocaml":
		return fmt.Sprintf("ocaml -e '%s'", body), nil
	case "f#":
		return fmt.Sprintf("fsharpi -e '%s'", body), nil
	case "crystal":
		return fmt.Sprintf("crystal eval '%s'", body), nil
	case "reason":
		return fmt.Sprintf("reason-cli -e '%s'", body), nil
	case "d":
		return fmt.Sprintf("dmd -run '%s'", body), nil
	case "solidity":
		return fmt.Sprintf("solc --bin '%s'", body), nil
	case "v":
		return fmt.Sprintf("v run '%s'", body), nil
	case "zig":
		return fmt.Sprintf("zig run '%s'", body), nil
	case "vala":
		return fmt.Sprintf("valac --pkg gtk+-3.0 '%s'", body), nil
	case "c", "gcc":
		return fmt.Sprintf("cat <<EOF > /tmp/tmp.c \n%s\nEOF && gcc /tmp/tmp.c -o /tmp/tmp.out && /tmp/tmp.out && rm /tmp/tmp.*", body), nil
	default:
		return "", fmt.Errorf("unsupported language: %s", lng)
	}
}
