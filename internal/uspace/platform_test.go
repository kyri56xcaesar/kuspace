package uspace

import (
	"context"
	"errors"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func int32p(n int32) *int32 { return &n }

func TestKubernetesReplicas(t *testing.T) {
	sel := func(app string) *metav1.LabelSelector {
		return &metav1.LabelSelector{MatchLabels: map[string]string{"app": app}}
	}
	pod := func(name, app string, ready bool, restarts int32, waiting string) *corev1.Pod {
		cs := corev1.ContainerStatus{Name: app, Ready: ready, RestartCount: restarts, Image: "kuspace:" + app}
		if waiting != "" {
			cs.State.Waiting = &corev1.ContainerStateWaiting{Reason: waiting}
		}

		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "kuspace", Labels: map[string]string{"app": app}},
			Spec:       corev1.PodSpec{NodeName: "node-1"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{cs}},
		}
	}
	client := fake.NewClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "frontapp", Namespace: "kuspace"},
			Spec: appsv1.DeploymentSpec{Replicas: int32p(2), Selector: sel("frontapp")}, Status: appsv1.DeploymentStatus{ReadyReplicas: 1}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "uspace", Namespace: "kuspace"},
			Spec: appsv1.StatefulSetSpec{Replicas: int32p(1), Selector: sel("uspace")}, Status: appsv1.StatefulSetStatus{ReadyReplicas: 1}},
		pod("frontapp-a", "frontapp", true, 0, ""),
		pod("frontapp-b", "frontapp", false, 7, "CrashLoopBackOff"),
		pod("uspace-0", "uspace", true, 1, ""),
		pod("other", "minio", true, 0, ""), // not selected by either
	)
	p := platformProbe{inCluster: true, namespace: "kuspace", kube: func() (kubernetes.Interface, error) { return client, nil }}
	d := p.deployment(t.Context())
	if d.Platform != "kubernetes" || d.Where != "namespace kuspace" || d.Error != "" || len(d.Services) != 2 {
		t.Fatalf("deployment: %+v", d)
	}
	front, us := d.Services[0], d.Services[1]
	if front.Name != "frontapp" || front.Kind != "Deployment" || front.Desired != 2 || front.Ready != 1 || len(front.Instances) != 2 {
		t.Errorf("frontapp: %+v", front)
	}
	if b := front.Instances[1]; b.Name != "frontapp-b" || b.Ready || b.Restarts != 7 || b.State != "CrashLoopBackOff" || b.Node != "node-1" {
		t.Errorf("crashing pod: %+v", b)
	}
	if us.Name != "uspace" || us.Kind != "StatefulSet" || us.Desired != 1 || us.Ready != 1 || us.Instances[0].Image != "kuspace:uspace" {
		t.Errorf("uspace: %+v", us)
	}

	// no access to the cluster: said so
	p.kube = func() (kubernetes.Interface, error) { return nil, errors.New("no service account token") }
	if d := p.deployment(t.Context()); d.Platform != "kubernetes" || !strings.Contains(d.Error, "no service account token") {
		t.Errorf("no cluster: %+v", d)
	}
}

func TestComposeReplicas(t *testing.T) {
	inspect := strings.Join([]string{
		`{"name":"/frontapp","service":"frontapp","state":"running","health":"","restarts":0,"startedAt":"2026-09-30T08:00:00Z","image":"kuspace-frontapp"}`,
		`{"name":"/minioth","service":"minioth","state":"running","health":"unhealthy","restarts":3,"startedAt":"2026-09-30T08:00:00Z","image":"kuspace-minioth"}`,
		`{"name":"/uspace","service":"uspace","state":"exited","health":"","restarts":0,"startedAt":"2026-09-30T08:00:00Z","image":"kuspace-uspace"}`,
	}, "\n")
	docker := func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case args[0] == "inspect" && strings.Contains(args[2], "com.docker.compose.project"): // uspace's own container
			if args[3] != "abc123" {
				return []byte("Error: No such object: " + args[3]), errors.New("exit status 1")
			}

			return []byte("docker-compose\n"), nil
		case args[0] == "ps":
			return []byte("c1\nc2\nc3\n"), nil
		case args[0] == "inspect":
			return []byte(inspect), nil
		}

		return nil, errors.New("unexpected")
	}
	p := platformProbe{docker: docker, hostname: "abc123"}
	d := p.deployment(t.Context())
	if d.Platform != "compose" || d.Where != "project docker-compose" || d.Error != "" || len(d.Services) != 3 {
		t.Fatalf("deployment: %+v", d)
	}
	want := map[string]struct {
		ready int
		state string
	}{"frontapp": {1, "running"}, "minioth": {0, "unhealthy"}, "uspace": {0, "exited"}}
	for _, s := range d.Services {
		w := want[s.Name]
		if s.Kind != "container" || s.Desired != 1 || s.Ready != w.ready || s.Instances[0].State != w.state || s.Instances[0].Name != s.Name {
			t.Errorf("%s: %+v", s.Name, s)
		}
	}

	// not in a compose container (go run): said so
	p.hostname = "laptop"
	if d := p.deployment(t.Context()); d.Platform != "" || !strings.Contains(d.Error, "outside kubernetes and docker compose") {
		t.Errorf("no platform: %+v", d)
	}
}
