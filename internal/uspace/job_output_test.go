package uspace

import (
	"context"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"
	"kyri56xcaesar/kuspace/pkg/fslite"
)

func newTestService(t *testing.T) *UService {
	t.Helper()
	cfg := ut.EnvConfig{
		ServiceConfig: ut.ServiceConfig{APIGinMode: "test"},
		FsliteConfig: ut.FsliteConfig{
			FslDB: "fsl_test.db", FslDBPath: t.TempDir() + "/", FslDBDriver: "sqlite3",
			FslDBMaxOpenConns: "4", FslDBMaxIdleConns: "2", FslDBMaxLifetime: "5",
			FslAccessKey: "admin", FslSecretKey: "adminpassword1",
		},
		StorageConfig: ut.StorageConfig{LocalVolumesDefaultCapacity: 1}, // GB: per-user quota in these tests
	}
	srv := &UService{config: cfg, fsl: fslite.NewFsLite(cfg)}
	if err := srv.fsl.CreateVolume(t.Context(), ut.Volume{Name: "vol1", CreatedAt: ut.CurrentTime()}); err != nil {
		t.Fatal(err)
	}

	return srv
}

func TestSaveJobOutputOverwrite(t *testing.T) {
	srv := newTestService(t)
	ctx := context.Background()
	const mb = int64(1_000_000)
	out := ut.Resource{Name: "result.csv", Vname: "vol1", VID: srv.volumeID(t.Context(), "vol1"), UID: 1001, GID: 1002,
		Perms: ut.DefaultFilePerms, Type: "file", Size: 600 * mb, CreatedAt: ut.CurrentTime()}

	if action, err := srv.saveJobOutput(ctx, out); err != nil || action != "created" {
		t.Fatalf("first run: %q, %v", action, err)
	}
	// the same job again, smaller result: update the record instead of failing
	out.Size = 200 * mb
	out.UID = 2002 // whoever ran it, the file's owner (1001) is charged
	if action, err := srv.saveJobOutput(ctx, out); err != nil || action == "created" {
		t.Fatalf("second run: %q, %v", action, err)
	}
	r, found, err := srv.lookupResource(t.Context(), "result.csv", "vol1")
	if err != nil || !found || r.Size != 200*mb || r.UID != 1001 {
		t.Fatalf("record after overwrite = %+v, %v (want size 200 MB, owner kept)", r, err)
	}
	// owner 1001 now holds 0.2 GB of a 1 GB quota: 0.7 GB more fits only if
	// the 0.4 GB difference was refunded
	if err := srv.fsl.ClaimSpace(ctx, 1001, "vol1", 700*mb, 1, true); err != nil {
		t.Errorf("overwrite didn't refund the size difference: %v", err)
	}
	if err := srv.fsl.ClaimSpace(ctx, 1001, "vol1", 200*mb, 1, true); err == nil {
		t.Error("quota not enforced after overwrite accounting")
	}
}
