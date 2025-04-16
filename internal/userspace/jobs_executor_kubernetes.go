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

func executeK8sJob(job ut.Job) {
	jobName := fmt.Sprintf("job-%d", job.Jid)

	jobSpec := buildK8sJob(
		jobName,
		job.Logic,
		[]string{"/bin/sh", "-c", job.Logic},
		map[string]string{
			"INPUT":  strings.Join(job.Input, ","),
			"OUTPUT": job.Output,
		},
		1, // parallelism
	)

	clientset := k.GetKubeClient() // from your config
	err := runJob(clientset, jobSpec, "default")
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
