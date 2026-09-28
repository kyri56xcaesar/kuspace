package fslite

import (
	"errors"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

func TestGroupVolumeQuotaIsTheGroups(t *testing.T) {
	fsl := newTestFsl(t)
	mustVolume(t, fsl, "team", 0)
	mustVolume(t, fsl, "personal", 0)

	gv, err := fsl.AssignGroupVolume(ctx, "team", 500, 2) // group 500, 2 GB
	if err != nil {
		t.Fatal(err)
	}
	if gv.Vname != "team" || gv.GID != 500 || gv.Quota != 2 {
		t.Fatalf("assigned = %+v", gv)
	}

	// two members share the group's 2 GB; their personal quota (1 GB) doesn't apply
	if err := fsl.InsertResource(ctx, file("m1", "team", 1001, gb+gb/2), 1, true); err != nil {
		t.Fatalf("member 1: %v", err)
	}
	if err := fsl.InsertResource(ctx, file("m2", "team", 1002, gb/2), 1, true); err != nil {
		t.Fatalf("member 2: %v", err)
	}
	if err := fsl.InsertResource(ctx, file("m3", "team", 1002, 1), 1, true); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("past the group quota: %v, want ErrQuotaExceeded", err)
	}
	if gv, _ = fsl.GroupVolume(ctx, "team"); gv.Usage != 2 {
		t.Errorf("group usage = %v GB, want 2", gv.Usage)
	}

	// and the group volume never touched the members' own accounts
	if err := fsl.InsertResource(ctx, file("p1", "personal", 1001, gb), 1, true); err != nil {
		t.Errorf("member's personal quota was charged for the group volume: %v", err)
	}
	uvs, err := fsl.selectUserVolumes(ctx, map[string]any{"uids": "1001"})
	if err != nil {
		t.Fatal(err)
	}
	for _, uv := range uvs.([]ut.UserVolume) {
		if uv.VID == gv.VID {
			t.Errorf("a personal claim exists on the group volume: %+v", uv)
		}
	}

	// deleting a member's file frees the group's space
	if err := fsl.Remove(ctx, ut.Resource{Name: "m2", Vname: "team"}); err != nil {
		t.Fatal(err)
	}
	if gv, _ = fsl.GroupVolume(ctx, "team"); gv.Usage != 1.5 {
		t.Errorf("group usage after a delete = %v GB, want 1.5", gv.Usage)
	}

	// changing the quota is allowed while it holds files
	if gv, err = fsl.AssignGroupVolume(ctx, "team", 500, 5); err != nil || gv.Quota != 5 {
		t.Errorf("quota change: %+v, %v", gv, err)
	}
}

func TestGroupVolumeAssignmentRules(t *testing.T) {
	fsl := newTestFsl(t)
	vid := mustVolume(t, fsl, "team", 0)
	mustVolume(t, fsl, "empty", 0)

	if _, err := fsl.AssignGroupVolume(ctx, defaultVolumeName, 500, 1); !errors.Is(err, ErrDefaultVolume) {
		t.Errorf("default volume: %v", err)
	}
	if _, err := fsl.AssignGroupVolume(ctx, "missing", 500, 1); err == nil {
		t.Error("missing volume assigned")
	}
	if _, err := fsl.AssignGroupVolume(ctx, "team", 0, 1); err == nil {
		t.Error("group 0 accepted")
	}

	// a volume with files can't change hands (their usage is already on someone's account)
	mustResource(t, fsl, ut.Resource{Name: "a.csv", Vname: "team", VID: vid, UID: 1001, GID: 1001, Type: "file"})
	if _, err := fsl.AssignGroupVolume(ctx, "team", 500, 1); !errors.Is(err, ErrVolumeInUse) {
		t.Errorf("volume with files: %v, want ErrVolumeInUse", err)
	}

	if _, err := fsl.AssignGroupVolume(ctx, "empty", 500, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := fsl.AssignGroupVolume(ctx, "empty", 600, 1); err != nil {
		t.Errorf("empty volume to another group: %v", err)
	}
	if gvs, _ := fsl.GroupVolumes(ctx, []int64{600}); len(gvs) != 1 || gvs[0].Vname != "empty" {
		t.Errorf("group 600's volumes = %+v", gvs)
	}
	if gvs, _ := fsl.GroupVolumes(ctx, []int64{500}); len(gvs) != 0 {
		t.Errorf("group 500 still has %+v", gvs)
	}

	if err := fsl.ReleaseGroupVolume(ctx, "empty"); err != nil {
		t.Fatal(err)
	}
	if _, err := fsl.GroupVolume(ctx, "empty"); !errors.Is(err, ErrNotGroupVolume) {
		t.Errorf("after release: %v", err)
	}
	if err := fsl.ReleaseGroupVolume(ctx, "empty"); !errors.Is(err, ErrNotGroupVolume) {
		t.Errorf("second release: %v", err)
	}

	// deleting the volume drops its assignment
	if _, err := fsl.AssignGroupVolume(ctx, "empty", 700, 1); err != nil {
		t.Fatal(err)
	}
	if err := fsl.RemoveVolume(ctx, ut.Volume{Name: "empty"}); err != nil {
		t.Fatal(err)
	}
	if gvs, _ := fsl.GroupVolumes(ctx, nil); len(gvs) != 0 {
		t.Errorf("assignment outlived its volume: %+v", gvs)
	}
}
