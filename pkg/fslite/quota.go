package fslite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

var (
	// ErrQuotaExceeded: the write would take the user past their quota.
	ErrQuotaExceeded = errors.New("storage quota exceeded")
	// ErrVolumeFull: the write would take the volume past its capacity.
	ErrVolumeFull = errors.New("volume is full")
)

// ClaimSpace charges size bytes to uid on the named volume, in one
// transaction. With enforce, it fails with ErrVolumeFull / ErrQuotaExceeded
// instead of charging past the volume's capacity or the user's quota; without
// it (e.g. job outputs that already exist) it only records the usage.
//
// A capacity or quota of 0 means unlimited. uid 0 (root) is never limited or
// tracked. A user without a claim on the volume gets one with defaultQuota GB.
func (fsl *FsLite) ClaimSpace(ctx context.Context, uid int64, volume string, size int64, defaultQuota float64, enforce bool) error {
	return fsl.adjustSpace(ctx, uid, volume, ut.SizeInGb(size), defaultQuota, enforce)
}

// ReleaseSpace gives size bytes back to uid on the named volume (never below 0).
func (fsl *FsLite) ReleaseSpace(ctx context.Context, uid int64, volume string, size int64) error {
	return fsl.adjustSpace(ctx, uid, volume, -ut.SizeInGb(size), 0, false)
}

func (fsl *FsLite) adjustSpace(ctx context.Context, uid int64, volume string, deltaGB, defaultQuota float64, enforce bool) error {
	if uid == 0 || deltaGB == 0 {
		return nil
	}
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit

	var vid int64
	var capacity, vusage float64
	err = tx.QueryRowContext(ctx, `SELECT vid, COALESCE(capacity, 0), COALESCE(usage, 0) FROM volumes WHERE name = ?`, volume).
		Scan(&vid, &capacity, &vusage)
	if err != nil {
		return fmt.Errorf("volume %q: %w", volume, err)
	}

	var quota, usage float64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(quota, 0), COALESCE(usage, 0) FROM user_volume WHERE vid = ? AND uid = ?`, vid, uid).
		Scan(&quota, &usage)
	if errors.Is(err, sql.ErrNoRows) {
		quota, usage = defaultQuota, 0
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_volume (vid, uid, usage, quota, updatedAt) VALUES (?, ?, 0, ?, ?)`,
			vid, uid, quota, ut.CurrentTime()); err != nil {
			return fmt.Errorf("create claim: %w", err)
		}
	} else if err != nil {
		return err
	}

	if enforce && deltaGB > 0 {
		if capacity > 0 && vusage+deltaGB > capacity {
			return ErrVolumeFull
		}
		if quota > 0 && usage+deltaGB > quota {
			return ErrQuotaExceeded
		}
	}

	now := ut.CurrentTime()
	if _, err := tx.ExecContext(ctx, `UPDATE user_volume SET usage = MAX(0, usage + ?), updatedAt = ? WHERE vid = ? AND uid = ?`,
		deltaGB, now, vid, uid); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE volumes SET usage = MAX(0, COALESCE(usage, 0) + ?) WHERE vid = ?`, deltaGB, vid); err != nil {
		return err
	}

	return tx.Commit()
}

// SetObjectSize records a new size (and modification time) for an existing
// resource, e.g. when a job overwrote it.
func (fsl *FsLite) SetObjectSize(ctx context.Context, name, volume string, size int64) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return err
	}
	now := ut.CurrentTime()
	res, err := db.ExecContext(ctx, `UPDATE resources SET size = ?, updatedAt = ?, accessedAt = ? WHERE name = ? AND vname = ?`,
		size, now, now, NormalizeName(name), volume)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}

	return nil
}
