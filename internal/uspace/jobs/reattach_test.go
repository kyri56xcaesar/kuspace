package jobs

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"kyri56xcaesar/kuspace/internal/model"
)

func TestDockerInspect(t *testing.T) {
	for out, want := range map[string]engineState{
		"running 0\n": engineRunning,
		"created 0":   engineRunning,
		"exited 0":    engineSucceeded,
		"exited 137":  engineFailed,
		"dead 1":      engineFailed,
	} {
		je := dockerExecutor("", fakeDocker(map[string]string{"inspect": out}))
		if got, err := je.inspect(t.Context(), model.Job{JID: 7}); got != want || err != nil {
			t.Errorf("%q: %v %v, want %v", out, got, err, want)
		}
	}
	gone := dockerExecutor("", fakeDocker(map[string]string{"inspect": "Error: No such object: kuspace-job-7"}, "inspect"))
	if got, err := gone.inspect(t.Context(), model.Job{JID: 7}); got != engineGone || err != nil {
		t.Errorf("no container: %v %v", got, err)
	}
	down := dockerExecutor("", fakeDocker(map[string]string{"inspect": "Cannot connect to the Docker daemon"}, "inspect"))
	if got, err := down.inspect(t.Context(), model.Job{JID: 7}); got != engineUnknown || err == nil {
		t.Errorf("engine down: %v %v (must not look gone)", got, err)
	}
}

func TestKubernetesInspect(t *testing.T) {
	job := func(jid int64, cond batchv1.JobConditionType) *batchv1.Job {
		j := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: k8sJobName(jid), Namespace: "ns"}}
		if cond != "" {
			j.Status.Conditions = []batchv1.JobCondition{{Type: cond, Status: corev1.ConditionTrue}}
		}

		return j
	}
	client := fake.NewClientset(job(1, ""), job(2, batchv1.JobComplete), job(3, batchv1.JobFailed))
	for jid, want := range map[int64]engineState{1: engineRunning, 2: engineSucceeded, 3: engineFailed, 4: engineGone} {
		if got, err := k8sState(t.Context(), client, "ns", jid); got != want || err != nil {
			t.Errorf("job %d: %v %v, want %v", jid, got, err, want)
		}
	}
}
