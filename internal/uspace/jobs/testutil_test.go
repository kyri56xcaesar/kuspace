package jobs

import (
	"context"
	"testing"
	"time"

	"kyri56xcaesar/kuspace/internal/model"
	"kyri56xcaesar/kuspace/internal/sqlite"
)

// openTestStore is a migrated jobs.db in a temporary directory.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	h := sqlite.NewDBHandler("jobs_test.db", t.TempDir()+"/", "sqlite3")
	if err := Open(t.Context(), &h, "4", "2", "5"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)

	return NewStore(&h)
}

// fakeStorage stands in for uspace's storage: outputs are "written" when a
// test says so.
type fakeStorage struct {
	written func(job model.Job) bool
}

func (f fakeStorage) DefaultVolume() string { return "vol1" }

func (f fakeStorage) PresignJobIO(context.Context, model.Resource, model.Resource, time.Duration) (string, string, error) {
	return "http://minio/in", "http://minio/out", nil
}

func (f fakeStorage) RecordOutput(context.Context, model.Job) (string, error) { return "recorded", nil }

func (f fakeStorage) OutputWrittenSince(_ context.Context, job model.Job, _ time.Time) bool {
	return f.written != nil && f.written(job)
}
