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
	wg.Wait()

	respondInFormat(c, c.Query("format"), out, "system_status.html")
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
