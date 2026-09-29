package frontendapp

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

// serviceStatus is one row of the admin "System" page.
type serviceStatus struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	Address   string `json:"address"`
	Up        bool   `json:"up"`
	Status    int    `json:"status"`
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error,omitempty"`
	// HasConfig is true when /admin/system-conf can show this service's settings.
	HasConfig bool `json:"hasConfig"`
	// Info is the service's account of itself (build, uptime, instance,
	// memory); nil when it didn't give one (minioth doesn't).
	Info *ut.ServiceInfo `json:"info,omitempty"`
	// Replicas are its instances as the platform sees them (nil when uspace
	// can't tell: no platform, or uspace down).
	Replicas *ut.ServiceReplicas `json:"replicas,omitempty"`
	Scaling  scaling             `json:"scaling"`
}

// scaling says whether a service can run as more than one instance.
type scaling struct {
	Can bool   `json:"can"`
	Why string `json:"why"`
}

// scalingOf is what kuspace's design allows each workload (the reasons
// are architectural: see the services' READMEs).
var scalingOf = map[string]scaling{
	"frontapp": {true, "stateless: sessions are signed cookies, so any instance serves any request"},
	"wss":      {false, "live job streams live in its memory: a job and those watching it must reach the same instance"},
	"uspace":   {false, "its SQLite databases (jobs, file records) take one writer, and a dispatcher runs its jobs in-process"},
	"minioth":  {false, "its SQLite database takes one writer"},
	"minio":    {true, "only as a distributed MinIO deployment (several drives and nodes), not by adding replicas"},
}

// otherWorkload is a workload of the deployment that isn't a kuspace
// service (MinIO, ...).
type otherWorkload struct {
	Replicas *ut.ServiceReplicas
	Scaling  scaling
}

const statusProbeTimeout = 3 * time.Second

// handleSysStatus probes every service's health endpoint in parallel and
// reports whether it answered, how fast, and with what status.
func (srv *HTTPService) handleSysStatus(c *gin.Context) {
	services := []struct {
		serviceStatus
		probe string
	}{
		{serviceStatus{Name: "frontapp", Role: "Web interface", Address: "self", HasConfig: true}, ""},
		{serviceStatus{Name: "minioth", Role: "Identity & tokens", Address: srv.minioth.base}, srv.minioth.base + "/v1/.well-known/ready"},
		{serviceStatus{Name: "uspace", Role: "Storage & jobs API", Address: srv.uspace.base, HasConfig: true}, srv.uspace.base + "/healthz"},
		{serviceStatus{Name: "wss", Role: "Live job updates", Address: srv.wss.base, HasConfig: true}, srv.wss.base + "/healthz"},
	}

	out := make([]serviceStatus, len(services))
	var wg sync.WaitGroup
	for i, s := range services {
		out[i] = s.serviceStatus
		out[i].Address = strings.TrimPrefix(out[i].Address, "http://")
		if s.probe == "" { // this process is answering the request, so it is up
			out[i].Up, out[i].Status = true, http.StatusOK

			continue
		}
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), statusProbeTimeout)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				out[i].Error = err.Error()

				return
			}
			start := time.Now()
			resp, err := http.DefaultClient.Do(req)
			out[i].LatencyMs = time.Since(start).Milliseconds()
			if err != nil {
				out[i].Error = "no response"

				return
			}
			_ = resp.Body.Close()
			out[i].Status = resp.StatusCode
			out[i].Up = resp.StatusCode >= 200 && resp.StatusCode < 300
		}(i, s.probe)
	}
	for i := range out {
		out[i].Scaling = scalingOf[out[i].Name]
		if info := srv.serviceInfo(c, out[i].Name); info != nil {
			wg.Add(1)
			go func() { defer wg.Done(); out[i].Info = info() }()
		}
	}
	var dep ut.Deployment
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*statusProbeTimeout)
		defer cancel()
		if err := srv.uspace.decode(ctx, request{method: http.MethodGet, path: "/api/v1/admin/replicas", token: userToken(c), target: "0::/"}, &dep); err != nil {
			dep = ut.Deployment{Error: "uspace did not answer, so the replicas can't be shown"}
		}
	}()
	wg.Wait()

	var others []otherWorkload
	for _, w := range dep.Services {
		matched := false
		for i := range out {
			if out[i].Name == w.Name {
				out[i].Replicas, matched = &w, true
			}
		}
		if !matched {
			others = append(others, otherWorkload{&w, scalingOf[w.Name]})
		}
	}
	if c.Query("format") == "json" {
		c.JSON(http.StatusOK, out)

		return
	}
	c.HTML(http.StatusOK, "system_status.html", gin.H{"services": out, "deployment": dep, "others": others})
}

// serviceInfo returns how to ask a service about itself (nil: it can't say).
func (srv *HTTPService) serviceInfo(c *gin.Context, service string) func() *ut.ServiceInfo {
	ask := func(u *upstream, r request) func() *ut.ServiceInfo {
		return func() *ut.ServiceInfo {
			ctx, cancel := context.WithTimeout(c.Request.Context(), statusProbeTimeout)
			defer cancel()
			var info ut.ServiceInfo
			if err := u.decode(ctx, r, &info); err != nil {
				return nil
			}

			return &info
		}
	}
	switch service {
	case "frontapp":
		return func() *ut.ServiceInfo {
			i := ut.Info("frontapp")

			return &i
		}
	case "uspace":
		return ask(srv.uspace, request{method: http.MethodGet, path: "/api/v1/admin/info", token: userToken(c), target: "0::/"})
	case "wss":
		return ask(srv.wss, request{method: http.MethodGet, path: "/info", svc: true})
	default:
		return nil
	}
}

// handleExecutorStatus shows the execution engines (uspace checks them):
// each one's state, which takes new jobs, and how many jobs ran where.
func (srv *HTTPService) handleExecutorStatus(c *gin.Context) {
	var st ut.EnginesStatus
	err := srv.uspace.decode(c.Request.Context(), request{
		method: http.MethodGet, path: "/api/v1/admin/executor", token: userToken(c), target: "0::/",
	}, &st)
	srv.renderEngines(c, st, err, "")
}

// handleExecutorSwitch sends new jobs to the engine named in the form, then
// shows the engines again (with the reason when uspace refused).
func (srv *HTTPService) handleExecutorSwitch(c *gin.Context) {
	engine := c.PostForm("engine")
	if engine == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "choose an engine"})

		return
	}
	var st ut.EnginesStatus
	err := srv.uspace.decode(c.Request.Context(), request{
		method: http.MethodPut, path: "/api/v1/admin/executor", token: userToken(c), target: "0::/",
		json: gin.H{"engine": engine},
	}, &st)
	notice := ""
	if err != nil {
		var ue *upstreamError
		if !errors.As(err, &ue) {
			srv.renderEngines(c, st, err, "")

			return
		}
		notice = "Not switched: " + ue.msg
		// show the engines as they are
		err = srv.uspace.decode(c.Request.Context(), request{
			method: http.MethodGet, path: "/api/v1/admin/executor", token: userToken(c), target: "0::/",
		}, &st)
	}
	srv.renderEngines(c, st, err, notice)
}

func (srv *HTTPService) renderEngines(c *gin.Context, st ut.EnginesStatus, err error, notice string) {
	if err != nil {
		if c.Query("format") == "json" {
			fail(c, "check the execution engines", err)

			return
		}
		notice = "uspace did not answer, so the engines can't be checked"
	}
	if c.Query("format") == "json" {
		c.JSON(http.StatusOK, st)

		return
	}
	c.HTML(http.StatusOK, "executor_status.html", gin.H{"st": st, "notice": notice, "failed": err != nil})
}
