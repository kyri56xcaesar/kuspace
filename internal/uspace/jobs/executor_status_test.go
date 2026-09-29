package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sversion "k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"kyri56xcaesar/kuspace/internal/config"
	"kyri56xcaesar/kuspace/internal/model"
)

// fakeDocker answers the docker CLI calls Status makes: args joined by " "
// map to (output, fail).
func fakeDocker(answers map[string]string, failing ...string) func(ctx context.Context, args ...string) ([]byte, error) {
	return func(_ context.Context, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		for _, f := range failing {
			if strings.HasPrefix(cmd, f) {
				return []byte(answers[f]), errors.New("exit status 1")
			}
		}
		for prefix, out := range answers {
			if strings.HasPrefix(cmd, prefix) {
				return []byte(out), nil
			}
		}

		return nil, errors.New("unexpected docker call: " + cmd)
	}
}

func dockerExecutor(network string, run func(context.Context, ...string) ([]byte, error)) JDockerExecutor {
	rt := newRuntimeWith(Deps{Config: config.EnvConfig{JobsConfig: config.JobsConfig{UspaceJobDockerNetwork: network}}})

	return JDockerExecutor{rt: rt, run: run}
}

func TestDockerStatus(t *testing.T) {
	ok := map[string]string{"version": "27.3.1\n", "network inspect": "kuspace-net\n"}
	st := dockerExecutor("kuspace-net", fakeDocker(ok)).Status(t.Context())
	if !st.Active || st.Engine != "docker" || st.Version != "27.3.1" || st.Where != "network kuspace-net" || st.Error != "" {
		t.Errorf("engine up: %+v", st)
	}

	down := map[string]string{"version": "Client: 27.3.1\nCannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?\n"}
	st = dockerExecutor("kuspace-net", fakeDocker(down, "version")).Status(t.Context())
	if st.Active || !strings.Contains(st.Error, "Cannot connect to the Docker daemon") {
		t.Errorf("engine down: %+v", st)
	}

	noNet := map[string]string{"version": "27.3.1", "network inspect": "Error response from daemon: network kuspace-net not found"}
	st = dockerExecutor("kuspace-net", fakeDocker(noNet, "network inspect")).Status(t.Context())
	if st.Active || !strings.Contains(st.Error, "network kuspace-net not found") {
		t.Errorf("network missing: %+v", st)
	}

	// no docker CLI at all
	st = dockerExecutor("", func(context.Context, ...string) ([]byte, error) {
		return nil, errors.New(`exec: "docker": executable file not found in $PATH`)
	}).Status(t.Context())
	if st.Active || !strings.Contains(st.Error, "executable file not found") || st.Where != "the default network" {
		t.Errorf("no CLI: %+v", st)
	}
}

func TestKubernetesStatus(t *testing.T) {
	client := fake.NewClientset()
	client.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &k8sversion.Info{GitVersion: "v1.31.2"}
	st := kubeStatus(t.Context(), client, "kuspace")
	if !st.Active || st.Version != "v1.31.2" || st.Where != "namespace kuspace" {
		t.Errorf("cluster up: %+v", st)
	}

	// the service account may not list jobs there
	client.PrependReactor("list", "jobs", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, &batchv1.JobList{}, errors.New(`jobs.batch is forbidden: User "system:serviceaccount:kuspace:uspace" cannot list resource "jobs"`)
	})
	st = kubeStatus(t.Context(), client, "kuspace")
	if st.Active || !strings.Contains(st.Error, "forbidden") || st.Version != "v1.31.2" {
		t.Errorf("no permission: %+v", st)
	}
}

// A job stays on the engine it recorded; one without an engine goes to the
// one taking new jobs.
func TestEngineSetForJob(t *testing.T) {
	e := newEngineSet(&stubEngine{name: "docker"}, &stubEngine{name: "kubernetes"})
	if err := e.Use("kubernetes"); err != nil {
		t.Fatal(err)
	}
	if x := e.forJob("docker"); x.Name() != "docker" {
		t.Errorf("a docker job is handled by %s", x.Name())
	}
	if x := e.forJob(""); x.Name() != "kubernetes" {
		t.Errorf("a job without an engine goes to %s, want the current one", x.Name())
	}
	if err := e.Use("nomad"); err == nil {
		t.Error("an unknown engine was accepted")
	}
}

// stubEngine is an engine whose status a test sets.
type stubEngine struct {
	name, err string
}

func (s *stubEngine) ExecuteJob(model.Job) error { return nil }
func (s *stubEngine) CancelJob(model.Job) error  { return nil }
func (s *stubEngine) Name() string               { return s.name }
func (s *stubEngine) Status(context.Context) model.ExecutorStatus {
	return model.ExecutorStatus{Engine: s.name, Active: s.err == "", Error: s.err}
}
