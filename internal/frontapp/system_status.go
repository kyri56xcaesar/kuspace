package frontendapp

import (
	"context"
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

// handleExecutorStatus shows the engine jobs run on (uspace checks it): the
// engine, whether it is active, and how many jobs ran where.
func (srv *HTTPService) handleExecutorStatus(c *gin.Context) {
	var st ut.ExecutorStatus
	err := srv.uspace.decode(c.Request.Context(), request{
		method: http.MethodGet, path: "/api/v1/admin/executor", token: userToken(c), target: "0::/",
	}, &st)
	if err != nil {
		if c.Query("format") == "json" {
			fail(c, "check the execution engine", err)

			return
		}
		st = ut.ExecutorStatus{Error: "uspace did not answer, so the engine can't be checked", CheckedAt: ut.CurrentTime()}
	}
	respondInFormat(c, c.Query("format"), st, "executor_status.html")
}
