package fslite

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

// newTestFsl builds an FsLite on a fresh SQLite file (in-memory SQLite gives
// every pooled connection its own empty database).
func newTestFsl(t *testing.T) *FsLite {
	t.Helper()
	fsl := NewFsLite(ut.EnvConfig{
		ServiceConfig: ut.ServiceConfig{APIGinMode: "test"},
		FsliteConfig: ut.FsliteConfig{
			FslDB:             "fsl_test.db",
			FslDBPath:         t.TempDir() + "/",
			FslDBDriver:       "sqlite3",
			FslDBMaxOpenConns: "4",
			FslDBMaxIdleConns: "2",
			FslDBMaxLifetime:  "5",
			FslAccessKey:      "admin",
			FslSecretKey:      "adminpassword1",
		},
	})
	t.Cleanup(fsl.Close)

	return &fsl
}

func mustVolume(t *testing.T, fsl *FsLite, name string, capacityGB float64) int64 {
	t.Helper()
	if err := fsl.CreateVolume(t.Context(), ut.Volume{Name: name, Capacity: capacityGB, CreatedAt: ut.CurrentTime()}); err != nil {
		t.Fatalf("create volume %s: %v", name, err)
	}
	v, err := fsl.SelectVolumes(t.Context(), map[string]any{"name": name})
	if err != nil {
		t.Fatalf("select volume %s: %v", name, err)
	}

	return v.(ut.Volume).VID
}

func mustResource(t *testing.T, fsl *FsLite, r ut.Resource) {
	t.Helper()
	now := ut.CurrentTime()
	r.CreatedAt, r.UpdatedAt, r.AccessedAt = now, now, now
	if r.Perms == "" {
		r.Perms = ut.DefaultFilePerms
	}
	if err := fsl.Insert(t.Context(), r); err != nil {
		t.Fatalf("insert %s: %v", r.Name, err)
	}
}

func lookup(t *testing.T, fsl *FsLite, name, volume string) (ut.Resource, bool) {
	t.Helper()
	r, err := fsl.SelectObjects(t.Context(), map[string]any{"name": name, "volume": volume})
	if err != nil {
		return ut.Resource{}, false
	}

	return r.(ut.Resource), true
}

const gb = int64(1_000_000_000)

var ctx = context.Background()

func TestClaimSpace(t *testing.T) {
	fsl := newTestFsl(t)
	mustVolume(t, fsl, "volume1", 0) // capacity 0 = unlimited

	// no claim yet: one is created with the default quota (2 GB)
	if err := fsl.ClaimSpace(ctx, 1001, "volume1", gb, 2, true); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if err := fsl.ClaimSpace(ctx, 1001, "volume1", gb, 2, true); err != nil {
		t.Fatalf("up to the quota: %v", err)
	}
	if err := fsl.ClaimSpace(ctx, 1001, "volume1", 1, 2, true); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("past the quota: got %v, want ErrQuotaExceeded", err)
	}
	// recording without enforcement (e.g. a finished job's output) still works
	if err := fsl.ClaimSpace(ctx, 1001, "volume1", gb, 2, false); err != nil {
		t.Fatalf("unenforced claim: %v", err)
	}
	// releases give space back, never below zero
	if err := fsl.ReleaseSpace(ctx, 1001, "volume1", 10*gb); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := fsl.ClaimSpace(ctx, 1001, "volume1", 2*gb, 2, true); err != nil {
		t.Fatalf("after release: %v", err)
	}
	// root is never limited
	if err := fsl.ClaimSpace(ctx, 0, "volume1", 1000*gb, 2, true); err != nil {
		t.Fatalf("root: %v", err)
	}
	// unknown volume
	if err := fsl.ClaimSpace(ctx, 1001, "nope", 1, 2, true); err == nil {
		t.Fatal("claim on a missing volume succeeded")
	}
}

func TestClaimSpaceVolumeCapacity(t *testing.T) {
	fsl := newTestFsl(t)
	mustVolume(t, fsl, "small", 1) // 1 GB volume

	if err := fsl.ClaimSpace(ctx, 1001, "small", gb, 0, true); err != nil { // quota 0 = unlimited
		t.Fatalf("fill the volume: %v", err)
	}
	if err := fsl.ClaimSpace(ctx, 1002, "small", 1, 0, true); !errors.Is(err, ErrVolumeFull) {
		t.Fatalf("past capacity: got %v, want ErrVolumeFull", err)
	}
}

func TestNamesAreNormalized(t *testing.T) {
	fsl := newTestFsl(t)
	vid := mustVolume(t, fsl, "volume1", 0)
	mustResource(t, fsl, ut.Resource{Name: "data.csv", Vname: "volume1", VID: vid, UID: 1001, GID: 1001, Type: "file"})

	if _, ok := lookup(t, fsl, "/data.csv", "volume1"); !ok {
		t.Fatal(`"data.csv" was not stored as "/data.csv"`)
	}
	if got := NormalizeName("//a/b"); got != "/a/b" {
		t.Errorf(`NormalizeName("//a/b") = %q`, got)
	}
}

func TestRenameStaysInItsVolume(t *testing.T) {
	fsl := newTestFsl(t)
	a := mustVolume(t, fsl, "vol-a", 0)
	b := mustVolume(t, fsl, "vol-b", 0)
	mustResource(t, fsl, ut.Resource{Name: "/x", Vname: "vol-a", VID: a, UID: 1, GID: 1, Type: "file"})
	mustResource(t, fsl, ut.Resource{Name: "/x", Vname: "vol-b", VID: b, UID: 2, GID: 2, Type: "file"})

	// move a:/x -> b:/y ; b:/x must be untouched (the WHERE used to ignore the volume)
	if err := fsl.Update(t.Context(), map[string]string{"name": "/x", "newname": "y", "volume": "vol-b", "oldvolume": "vol-a"}); err != nil {
		t.Fatalf("move: %v", err)
	}
	moved, ok := lookup(t, fsl, "/y", "vol-b")
	if !ok {
		t.Fatal("moved resource not found at b:/y")
	}
	if moved.VID != b {
		t.Errorf("moved resource vid = %d, want %d (vid must follow the volume)", moved.VID, b)
	}
	if _, ok := lookup(t, fsl, "/x", "vol-b"); !ok {
		t.Error("b:/x was renamed too")
	}
}

func TestSelectByIDs(t *testing.T) {
	fsl := newTestFsl(t)
	vid := mustVolume(t, fsl, "volume1", 0)
	mustResource(t, fsl, ut.Resource{Name: "/a", Vname: "volume1", VID: vid, UID: 1, GID: 1, Type: "file", Size: 7})
	r, _ := lookup(t, fsl, "/a", "volume1")

	// used to SELECT one column and scan fourteen: chmod/chown/chgrp all failed
	res, err := fsl.SelectObjects(t.Context(), map[string]any{"rids": strconv.FormatInt(r.RID, 10)})
	if err != nil {
		t.Fatalf("select by rid: %v", err)
	}
	got := res.([]ut.Resource)
	if len(got) != 1 || got[0].Name != "/a" || got[0].Size != 7 {
		t.Fatalf("select by rid = %+v", got)
	}
}

func TestForeignKeys(t *testing.T) {
	fsl := newTestFsl(t)
	vid := mustVolume(t, fsl, "volume1", 0)

	// a resource must reference an existing volume
	if err := fsl.Insert(t.Context(), ut.Resource{Name: "/orphan", Vname: "volume1", VID: 999, UID: 1, Perms: ut.DefaultFilePerms}); err == nil {
		t.Fatal("inserted a resource pointing at a missing volume")
	}
	// deleting a volume removes its resources (ON DELETE CASCADE)
	mustResource(t, fsl, ut.Resource{Name: "/f", Vname: "volume1", VID: vid, UID: 1, GID: 1, Type: "file"})
	if err := fsl.RemoveVolume(t.Context(), ut.Volume{Name: "volume1", VID: vid}); err != nil {
		t.Fatalf("remove volume: %v", err)
	}
	if _, ok := lookup(t, fsl, "/f", "volume1"); ok {
		t.Error("resource survived its volume's deletion")
	}
}

func TestConcurrentSameNameInserts(t *testing.T) {
	fsl := newTestFsl(t)
	vid := mustVolume(t, fsl, "volume1", 0)

	const n = 20
	errs := make(chan error, n)
	var start sync.WaitGroup
	start.Add(1)
	for i := range n {
		go func() {
			start.Wait()
			now := ut.CurrentTime()
			err := fsl.Insert(t.Context(), ut.Resource{Name: "race.txt", Vname: "volume1", VID: vid, UID: int64(1000 + i),
				GID: 1, Perms: ut.DefaultFilePerms, Type: "file", CreatedAt: now, UpdatedAt: now, AccessedAt: now})
			errs <- err
		}()
	}
	start.Done()
	ok := 0
	for range n {
		err := <-errs
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, ErrResourceExists):
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d concurrent inserts of one name succeeded, want exactly 1", ok)
	}
}

// withLocality turns on file storage under a temp dir. fsliteDataPath is a
// package variable, so these tests must not run in parallel.
func withLocality(t *testing.T, fsl *FsLite, volumes ...string) {
	t.Helper()
	old := fsliteDataPath
	fsliteDataPath = t.TempDir()
	t.Cleanup(func() { fsliteDataPath = old })
	fsl.config.FslLocality = true
	for _, v := range volumes {
		if err := os.MkdirAll(fsliteDataPath+"/"+v, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLocalDownloadAndStat(t *testing.T) {
	fsl := newTestFsl(t)
	vid := mustVolume(t, fsl, "volume1", 0)

	var r any = &ut.Resource{Name: "a.txt", Vname: "volume1"}
	if _, err := fsl.Download(t.Context(), &r); err == nil {
		t.Error("download without locality succeeded (fslite holds no data then)")
	}

	withLocality(t, fsl, "volume1")
	mustResource(t, fsl, ut.Resource{Name: "a.txt", Vname: "volume1", VID: vid, UID: 1, GID: 1, Type: "file",
		Reader: strings.NewReader("hello")})

	// uspace passes a *ut.Resource and reads the reader off it
	res := &ut.Resource{Name: "a.txt", Vname: "volume1"}
	r = res
	release, err := fsl.Download(t.Context(), &r)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	body, _ := io.ReadAll(res.Reader)
	release()
	if string(body) != "hello" || res.Size != 5 {
		t.Errorf("downloaded %q (size %d)", body, res.Size)
	}

	info, err := fsl.Stat(t.Context(), ut.Resource{Name: "a.txt", Vname: "volume1"})
	if err != nil || info.(os.FileInfo).Size() != 5 {
		t.Errorf("stat = %v, %v", info, err)
	}
}

func TestCreateVolumeExists(t *testing.T) {
	fsl := newTestFsl(t)
	mustVolume(t, fsl, "volume1", 0)
	if err := fsl.CreateVolume(t.Context(), ut.Volume{Name: "volume1", CreatedAt: ut.CurrentTime()}); !errors.Is(err, ErrVolumeExists) {
		t.Errorf("second create = %v, want ErrVolumeExists", err)
	}
}
