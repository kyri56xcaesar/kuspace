package utils

import (
	"testing"
	"time"
)

func TestInfo(t *testing.T) {
	i := Info("uspace")
	if i.Service != "uspace" || i.Commit == "" || i.GoVersion == "" || i.Instance == "" || i.Goroutines == 0 || i.HeapBytes == 0 {
		t.Errorf("info: %+v", i)
	}
	if _, err := time.Parse(time.RFC3339, i.StartedAt); err != nil || i.UptimeSec < 0 {
		t.Errorf("started %q uptime %d: %v", i.StartedAt, i.UptimeSec, err)
	}
	t.Logf("commit %q built %q", i.Commit, i.BuiltAt) // go test -ldflags "-X ...buildCommit=..." shows the stamp
}
