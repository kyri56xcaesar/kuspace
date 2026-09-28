package fslite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

/*
	Group volumes

	A volume can belong to one group. Its members share it: what they store
	there is charged to the group's quota on that volume, never to their
	personal (user_volume) quotas, and the volume's other quotas don't apply.
	Who may write there (members) is uspace's decision; fslite keeps the
	assignment and the accounting (ClaimSpace/ReleaseSpace branch on it).
*/

var (
	// ErrVolumeInUse is returned when a volume holds files, so it can't change hands.
	ErrVolumeInUse = errors.New("volume holds files")
	// ErrDefaultVolume is returned for the default volume: it is every user's, never a group's.
	ErrDefaultVolume = errors.New("the default volume can't be a group volume")
	// ErrNotGroupVolume is returned when the volume belongs to no group.
	ErrNotGroupVolume = errors.New("not a group volume")
)

const groupVolumeSchema = `
	CREATE TABLE IF NOT EXISTS group_volume (
	    vid INTEGER PRIMARY KEY,
	    gid INTEGER NOT NULL,
	    usage REAL NOT NULL DEFAULT 0,
	    quota REAL NOT NULL DEFAULT 0,
	    updatedAt DATETIME,
	    FOREIGN KEY (vid) REFERENCES volumes(vid) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_group_volume_gid ON group_volume(gid);
`

// AssignGroupVolume gives the named volume to group gid with quotaGB (0 =
// only the volume's capacity limits it), or changes the quota of a volume
// the group already has. A volume that holds files can't change hands.
func (fsl *FsLite) AssignGroupVolume(ctx context.Context, volume string, gid int64, quotaGB float64) (ut.GroupVolume, error) {
	if volume == defaultVolumeName || volume == fsl.config.MinioDefaultBucket {
		return ut.GroupVolume{}, ErrDefaultVolume
	}
	if gid <= 0 || quotaGB < 0 {
		return ut.GroupVolume{}, errors.New("group id must be positive and the quota not negative")
	}
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return ut.GroupVolume{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ut.GroupVolume{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var vid int64
	if err := tx.QueryRowContext(ctx, `SELECT vid FROM volumes WHERE name = ?`, volume).Scan(&vid); err != nil {
		return ut.GroupVolume{}, fmt.Errorf("volume %q: %w", volume, err)
	}
	owner, assigned, err := groupOf(ctx, tx, vid)
	if err != nil {
		return ut.GroupVolume{}, err
	}
	if !assigned || owner != gid {
		// changing hands: only while empty, or the usage already charged
		// (to users or to the previous group) would land on the wrong account
		var files int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources WHERE vid = ?`, vid).Scan(&files); err != nil {
			return ut.GroupVolume{}, err
		}
		if files > 0 {
			return ut.GroupVolume{}, ErrVolumeInUse
		}
	}
	now := ut.CurrentTime()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO group_volume (vid, gid, usage, quota, updatedAt) VALUES (?, ?, 0, ?, ?)
		ON CONFLICT(vid) DO UPDATE SET gid = excluded.gid, quota = excluded.quota, updatedAt = excluded.updatedAt`,
		vid, gid, quotaGB, now); err != nil {
		return ut.GroupVolume{}, err
	}
	// personal claims on a shared volume would never be charged: drop them
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_volume WHERE vid = ?`, vid); err != nil {
		return ut.GroupVolume{}, err
	}
	if err := tx.Commit(); err != nil {
		return ut.GroupVolume{}, err
	}

	return fsl.GroupVolume(ctx, volume)
}

// ReleaseGroupVolume makes an empty group volume an ordinary volume again.
func (fsl *FsLite) ReleaseGroupVolume(ctx context.Context, volume string) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return err
	}
	var files int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources r JOIN volumes v ON v.vid = r.vid WHERE v.name = ?`, volume).
		Scan(&files); err != nil {
		return err
	}
	if files > 0 {
		return ErrVolumeInUse
	}
	res, err := db.ExecContext(ctx, `DELETE FROM group_volume WHERE vid = (SELECT vid FROM volumes WHERE name = ?)`, volume)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotGroupVolume
	}

	return nil
}

// GroupVolume returns the named volume's group assignment, or
// ErrNotGroupVolume.
func (fsl *FsLite) GroupVolume(ctx context.Context, volume string) (ut.GroupVolume, error) {
	gvs, err := fsl.queryGroupVolumes(ctx, `WHERE v.name = ?`, volume)
	if err != nil {
		return ut.GroupVolume{}, err
	}
	if len(gvs) == 0 {
		return ut.GroupVolume{}, ErrNotGroupVolume
	}

	return gvs[0], nil
}

// GroupVolumes lists the group volumes of the given groups (all of them
// when gids is empty).
func (fsl *FsLite) GroupVolumes(ctx context.Context, gids []int64) ([]ut.GroupVolume, error) {
	if len(gids) == 0 {
		return fsl.queryGroupVolumes(ctx, "")
	}
	where := "WHERE g.gid IN (?" + repeatPlaceholders(len(gids)-1) + ")"
	args := make([]any, len(gids))
	for i, g := range gids {
		args[i] = g
	}

	return fsl.queryGroupVolumes(ctx, where, args...)
}

func (fsl *FsLite) queryGroupVolumes(ctx context.Context, where string, args ...any) ([]ut.GroupVolume, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT g.vid, v.name, g.gid, g.usage, g.quota, COALESCE(g.updatedAt, '')
		FROM group_volume g JOIN volumes v ON v.vid = g.vid `+where+` ORDER BY v.name`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	gvs := []ut.GroupVolume{}
	for rows.Next() {
		var gv ut.GroupVolume
		if err := rows.Scan(&gv.VID, &gv.Vname, &gv.GID, &gv.Usage, &gv.Quota, &gv.UpdatedAt); err != nil {
			return nil, err
		}
		gvs = append(gvs, gv)
	}

	return gvs, rows.Err()
}

// groupOf reports which group owns volume vid, if any.
func groupOf(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, vid int64) (int64, bool, error) {
	var gid int64
	err := q.QueryRowContext(ctx, `SELECT gid FROM group_volume WHERE vid = ?`, vid).Scan(&gid)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, false, nil
	case err != nil:
		return 0, false, err
	}

	return gid, true, nil
}

func repeatPlaceholders(n int) string {
	out := make([]byte, 0, 2*n)
	for range n {
		out = append(out, ',', '?')
	}

	return string(out)
}
