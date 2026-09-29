package utils

/*
	What a running service says about itself on the admin System page: the
	build (version, commit, when), since when it runs, which instance
	answered, and its memory. The images stamp the build at link time:

	  go build -ldflags "-X kyri56xcaesar/kuspace/internal/utils.buildCommit=<sha>
	                     -X kyri56xcaesar/kuspace/internal/utils.buildTime=<RFC 3339>"

	(build/Dockerfile.*, from the GIT_COMMIT and BUILD_TIME build arguments
	that make images and kuspacectl pass). A plain go build in the repository
	falls back to the commit Go records itself; otherwise it is "unknown".
*/

import (
	"os"
	"runtime"
	"runtime/debug"
	"time"
)

var (
	buildVersion = "1.0.0-dev"
	buildCommit  = ""
	buildTime    = ""
	startedAt    = time.Now()
)

// ServiceInfo is a service's own account of itself.
type ServiceInfo struct {
	Service   string `json:"service"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`            // short hash, "+dirty" when built from uncommitted changes
	BuiltAt   string `json:"builtAt,omitempty"` // RFC 3339
	GoVersion string `json:"goVersion"`
	StartedAt string `json:"startedAt"` // RFC 3339
	UptimeSec int64  `json:"uptimeSec"`
	// Instance is the host name: the container or pod that answered.
	Instance   string `json:"instance"`
	Goroutines int    `json:"goroutines"`
	HeapBytes  uint64 `json:"heapBytes"`
}

// Info describes this process as service.
func Info(service string) ServiceInfo {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	host, _ := os.Hostname()

	return ServiceInfo{
		Service: service, Version: buildVersion, Commit: commit(), BuiltAt: buildTime,
		GoVersion: runtime.Version(), StartedAt: startedAt.UTC().Format(time.RFC3339),
		UptimeSec: int64(time.Since(startedAt).Seconds()), Instance: host,
		Goroutines: runtime.NumGoroutine(), HeapBytes: ms.HeapAlloc,
	}
}

// commit is the stamped commit, or the one Go recorded, or "unknown".
func commit() string {
	if buildCommit != "" {
		return buildCommit
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	var rev, dirty string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "+dirty"
			}
		}
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if rev == "" {
		return "unknown"
	}

	return rev + dirty
}
