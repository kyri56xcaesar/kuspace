package fslite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

/*
	Personal quotas: a user's claim on a volume (user_volume). Usage is
	computed from the records (see quota.go); the claim holds the quota. A
	user without a claim gets the default quota on first write. Group
	volumes have no personal quotas: the group's quota applies there.
*/

// ErrSharedVolume is returned for personal quotas on a group volume.
var ErrSharedVolume = fmt.Errorf("a group volume has the group's quota, not personal ones (%w)", ut.ErrInvalid)

// UserVolumes lists users' claims with their computed usage and the volume
// name, optionally only those of uids and/or on the named volume.
func (fsl *FsLite) UserVolumes(ctx context.Context, uids []int64, volume string) ([]ut.UserVolume, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []any{}
	if len(uids) > 0 {
		where = append(where, "u.uid IN (?"+repeatPlaceholders(len(uids)-1)+")")
		for _, uid := range uids {
			args = append(args, uid)
		}
	}
	if volume != "" {
		where = append(where, "v.name = ?")
		args = append(args, volume)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT u.vid, v.name, u.uid, u.usage, COALESCE(u.quota, 0), COALESCE(u.updatedAt, '')
		FROM user_volume_usage u JOIN volumes v ON v.vid = u.vid
		WHERE `+strings.Join(where, " AND ")+` ORDER BY v.name, u.uid`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	uvs := []ut.UserVolume{}
	for rows.Next() {
		var uv ut.UserVolume
		if err := rows.Scan(&uv.VID, &uv.Vname, &uv.UID, &uv.Usage, &uv.Quota, &uv.UpdatedAt); err != nil {
			return nil, err
		}
		uvs = append(uvs, uv)
	}

	return uvs, rows.Err()
}

// SetUserQuota sets uid's quota (GB, 0 = unlimited) on the named volume,
// creating the claim if needed.
func (fsl *FsLite) SetUserQuota(ctx context.Context, volume string, uid int64, quotaGB float64) (ut.UserVolume, error) {
	if uid <= 0 || quotaGB < 0 {
		return ut.UserVolume{}, fmt.Errorf("uid must be positive and the quota not negative (%w)", ut.ErrInvalid)
	}
	err := fsl.immediate(ctx, func(q querier) error {
		var vid int64
		if err := q.QueryRowContext(ctx, `SELECT vid FROM volumes WHERE name = ?`, volume).Scan(&vid); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: %s", ErrVolumeNotFound, volume)
			}

			return err
		}
		if _, shared, err := groupOf(ctx, q, vid); err != nil {
			return err
		} else if shared {
			return ErrSharedVolume
		}
		_, err := q.ExecContext(ctx, `
			INSERT INTO user_volume (vid, uid, usage, quota, updatedAt) VALUES (?, ?, 0, ?, ?)
			ON CONFLICT(vid, uid) DO UPDATE SET quota = excluded.quota, updatedAt = excluded.updatedAt`,
			vid, uid, quotaGB, ut.CurrentTime())

		return err
	})
	if err != nil {
		return ut.UserVolume{}, err
	}
	uvs, err := fsl.UserVolumes(ctx, []int64{uid}, volume)
	if err != nil || len(uvs) == 0 {
		return ut.UserVolume{}, errors.Join(err, errors.New("claim not found after update"))
	}

	return uvs[0], nil
}

// ResetUserQuota removes uid's claim on the named volume: the default
// quota applies again on the next write. Their files are untouched.
func (fsl *FsLite) ResetUserQuota(ctx context.Context, volume string, uid int64) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return err
	}
	res, err := db.ExecContext(ctx, `DELETE FROM user_volume WHERE uid = ? AND vid = (SELECT vid FROM volumes WHERE name = ?)`, uid, volume)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("user %d has no quota on %s (%w)", uid, volume, ut.ErrNotFound)
	}

	return nil
}
