package uspace

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sversion "k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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
	jm := &JobManager{srv: &UService{config: ut.EnvConfig{JobsConfig: ut.JobsConfig{UspaceJobDockerNetwork: network}}}}

	return JDockerExecutor{jm: jm, run: run}
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

func TestExecutorStatusEndpoint(t *testing.T) {
	a := newAPIHarness(t, 1)
	a.withJobsDB(1001, 2002, 2002)
	if err := a.srv.markJobEngine(t.Context(), 1, "docker"); err != nil {
		t.Fatal(err)
	}
	jm := &JobManager{srv: a.srv}
	jm.executor = JDockerExecutor{jm: jm, run: fakeDocker(map[string]string{"version": "27.3.1"})}
	a.srv.jdp = JobDispatcherImpl{Manager: *jm}

	rec := a.do(http.MethodGet, "/api/v1/admin/executor", "::/", "0:0", nil, "")
	var st ut.ExecutorStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &st); rec.Code != http.StatusOK || err != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if !st.Active || st.Engine != "docker" || st.JobsByEngine["docker"] != 1 || st.JobsByEngine[""] != 2 {
		t.Errorf("status: %+v", st)
	}
}
