package uspace

/*
	Where kuspace is deployed and how many instances each service runs, for
	the admin System page (read-only: GET /admin/replicas).

	  kubernetes      uspace runs in a pod (KUBERNETES_SERVICE_HOST): the
	                  Deployments and StatefulSets of its namespace, their
	                  desired/ready replicas and their pods
	  docker compose  uspace runs in a compose container (the docker socket
	                  it uses for jobs): the containers of its compose project
	  neither         e.g. go run: nothing to show, and it says so
*/

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	k "kyri56xcaesar/kuspace/internal/uspace/kubernetes"
	ut "kyri56xcaesar/kuspace/internal/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"kyri56xcaesar/kuspace/internal/model"
)

// platformProbe finds the deployment; its fields let tests stand in.
type platformProbe struct {
	inCluster bool
	namespace string
	kube      func() (kubernetes.Interface, error)
	docker    func(ctx context.Context, args ...string) ([]byte, error)
	hostname  string // the container uspace runs in (compose)
}

func (srv *UService) platformProbe() platformProbe {
	host, _ := os.Hostname()

	return platformProbe{
		inCluster: os.Getenv("KUBERNETES_SERVICE_HOST") != "",
		namespace: srv.config.Namespace,
		kube:      func() (kubernetes.Interface, error) { return k.GetKubeClient() },
		docker: func(ctx context.Context, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		},
		hostname: host,
	}
}

// platformTimeout bounds asking the platform.
const platformTimeout = 10 * time.Second

// lastLine is the last non-empty line of a command's output (where the CLI
// tools put the error).
func lastLine(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	return strings.TrimSpace(lines[len(lines)-1])
}

var errNoPlatform = errors.New("uspace runs outside kubernetes and docker compose (e.g. go run): there are no replicas to show")

// deployment reports the platform and its services' replicas.
func (p platformProbe) deployment(ctx context.Context) model.Deployment {
	ctx, cancel := context.WithTimeout(ctx, platformTimeout)
	defer cancel()
	d := model.Deployment{CheckedAt: ut.CurrentTime()}
	var err error
	switch {
	case p.inCluster:
		d.Platform, d.Where = "kubernetes", "namespace "+p.namespace
		d.Services, err = p.kubeServices(ctx)
	default:
		var project string
		if project, err = p.composeProject(ctx); err == nil {
			d.Platform, d.Where = "compose", "project "+project
			d.Services, err = p.composeServices(ctx, project)
		}
	}
	if err != nil {
		d.Error = err.Error()
	}

	return d
}

// ---------------------------------------------------------------- kubernetes

func (p platformProbe) kubeServices(ctx context.Context) ([]model.ServiceReplicas, error) {
	client, err := p.kube()
	if err != nil {
		return nil, err
	}
	var out []model.ServiceReplicas
	deps, err := client.AppsV1().Deployments(p.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, d := range deps.Items {
		s := model.ServiceReplicas{Name: d.Name, Kind: "Deployment", Ready: int(d.Status.ReadyReplicas), Desired: 1}
		if d.Spec.Replicas != nil {
			s.Desired = int(*d.Spec.Replicas)
		}
		s.Instances, err = p.pods(ctx, client, d.Spec.Selector.MatchLabels)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	sets, err := client.AppsV1().StatefulSets(p.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, st := range sets.Items {
		s := model.ServiceReplicas{Name: st.Name, Kind: "StatefulSet", Ready: int(st.Status.ReadyReplicas), Desired: 1}
		if st.Spec.Replicas != nil {
			s.Desired = int(*st.Spec.Replicas)
		}
		s.Instances, err = p.pods(ctx, client, st.Spec.Selector.MatchLabels)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return out, nil
}

func (p platformProbe) pods(ctx context.Context, client kubernetes.Interface, match map[string]string) ([]model.Instance, error) {
	pods, err := client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(match).String()})
	if err != nil {
		return nil, err
	}
	out := make([]model.Instance, 0, len(pods.Items))
	for _, pod := range pods.Items {
		in := model.Instance{Name: pod.Name, State: strings.ToLower(string(pod.Status.Phase)), Node: pod.Spec.NodeName, Ready: true}
		if pod.Status.StartTime != nil {
			in.StartedAt = pod.Status.StartTime.UTC().Format(time.RFC3339)
		}
		if len(pod.Status.ContainerStatuses) == 0 {
			in.Ready = false
		}
		for _, c := range pod.Status.ContainerStatuses {
			in.Restarts += int(c.RestartCount)
			in.Ready = in.Ready && c.Ready
			if in.Image == "" {
				in.Image = c.Image
			}
			if c.State.Waiting != nil { // CrashLoopBackOff, ImagePullBackOff, ...
				in.State = c.State.Waiting.Reason
			}
		}
		out = append(out, in)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return out, nil
}

// ---------------------------------------------------------------- compose

// composeProject is the compose project of the container uspace runs in.
func (p platformProbe) composeProject(ctx context.Context) (string, error) {
	out, err := p.docker(ctx, "inspect", "--format", `{{index .Config.Labels "com.docker.compose.project"}}`, p.hostname)
	project := strings.TrimSpace(string(out))
	if err != nil || project == "" || project == "<no value>" {
		return "", errNoPlatform
	}

	return project, nil
}

// composeContainer is what docker inspect says about one container.
type composeContainer struct {
	Name         string `json:"name"`
	Service      string `json:"service"`
	State        string `json:"state"`
	Health       string `json:"health"`
	RestartCount int    `json:"restarts"`
	StartedAt    string `json:"startedAt"`
	Image        string `json:"image"`
}

const composeFormat = `{"name":{{json .Name}},"service":{{json (index .Config.Labels "com.docker.compose.service")}},` +
	`"state":{{json .State.Status}},"health":{{if .State.Health}}{{json .State.Health.Status}}{{else}}""{{end}},` +
	`"restarts":{{.RestartCount}},"startedAt":{{json .State.StartedAt}},"image":{{json .Config.Image}}}`

func (p platformProbe) composeServices(ctx context.Context, project string) ([]model.ServiceReplicas, error) {
	ids, err := p.docker(ctx, "ps", "--all", "--quiet", "--filter", "label=com.docker.compose.project="+project)
	if err != nil {
		return nil, errors.New("listing the compose containers: " + lastLine(ids))
	}
	idList := strings.Fields(string(ids))
	if len(idList) == 0 {
		return nil, nil
	}
	out, err := p.docker(ctx, append([]string{"inspect", "--format", composeFormat}, idList...)...)
	if err != nil {
		return nil, errors.New("inspecting the compose containers: " + lastLine(out))
	}
	byService := map[string]*model.ServiceReplicas{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		var c composeContainer
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, err
		}
		s, ok := byService[c.Service]
		if !ok {
			s = &model.ServiceReplicas{Name: c.Service, Kind: "container"}
			byService[c.Service] = s
		}
		ready := c.State == "running" && (c.Health == "" || c.Health == "healthy")
		state := c.State
		if c.Health != "" && c.State == "running" {
			state = c.Health
		}
		s.Desired++ // compose runs what it was asked to: every container counts
		if ready {
			s.Ready++
		}
		s.Instances = append(s.Instances, model.Instance{Name: strings.TrimPrefix(c.Name, "/"), State: state, Ready: ready,
			Restarts: c.RestartCount, StartedAt: c.StartedAt, Image: c.Image})
	}
	services := make([]model.ServiceReplicas, 0, len(byService))
	for _, s := range byService {
		services = append(services, *s)
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })

	return services, nil
}
