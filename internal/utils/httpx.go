package utils

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

/*
	Behaviour at the edges, shared by the services.

	Timeouts bound each phase instead of the whole exchange: connecting,
	TLS and waiting for response headers are short, while a body may stream
	for as long as the caller's context allows (a 2 GB download must not
	be cut at an arbitrary total, a dead peer must not hang a request).
*/

// NewTransport is an http.Transport with bounded connect, TLS and
// response-header waits.
func NewTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   16,
		ForceAttemptHTTP2:     true,
	}
}

// ServerTimeouts bounds the parts of a server exchange that don't stream:
// reading request headers, and idle keep-alive connections. (No read or
// write timeout: uploads, downloads and websockets stream.)
func ServerTimeouts(s *http.Server) *http.Server {
	s.ReadHeaderTimeout = 5 * time.Second
	s.IdleTimeout = 120 * time.Second

	return s
}

// Check is one readiness condition (a database answers, a dependency is
// reachable).
type Check func(ctx context.Context) error

// ReadinessTimeout bounds each readiness check.
const ReadinessTimeout = 2 * time.Second

// Ready runs the checks in parallel, each within ReadinessTimeout, and
// reports 200 when all pass or 503 with what failed.
func Ready(ctx context.Context, checks map[string]Check) (int, map[string]string) {
	results := make(map[string]string, len(checks))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, check := range checks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, ReadinessTimeout)
			defer cancel()
			res := "ok"
			if err := check(cctx); err != nil {
				res = err.Error()
			}
			mu.Lock()
			results[name] = res
			mu.Unlock()
		}()
	}
	wg.Wait()
	for _, r := range results {
		if r != "ok" {
			return http.StatusServiceUnavailable, results
		}
	}

	return http.StatusOK, results
}

// Reachable checks that url answers with a 2xx within the context.
func Reachable(client *http.Client, url string) Check {
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return ErrUnavailable
		}
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return ErrUnavailable
		}

		return nil
	}
}
