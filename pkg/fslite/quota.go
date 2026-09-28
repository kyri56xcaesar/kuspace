package fslite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

/*
	Usage and quotas

	Usage is not stored: it is the sum of the sizes of the resources on a
	volume (for a user: those they own; for a group volume: all of them).
	The views below compute it for the read queries, and the quota checks
	compute it inside the transaction that inserts the record. There used to
	be usage counters, adjusted by separate claim/release calls; any failure
	or crash between a record change and its counter update left them wrong
	for good.

	A capacity or quota of 0 means unlimited. Quotas and capacities are GB
	(10^9 bytes); sizes are bytes. uid 0 (root) is never limited.
*/

var (
	// ErrQuotaExceeded is returned when a write would take the owner (or
	// the group, on a group volume) past its quota.
	ErrQuotaExceeded = fmt.Errorf("storage quota exceeded (%w)", ut.ErrNoSpace)
	// ErrVolumeFull is returned when a write would take the volume past its capacity.
	ErrVolumeFull = fmt.Errorf("volume is full (%w)", ut.ErrNoSpace)
)

// usageSchema: the views the read queries use, and the index the sums use.
// Views are re-created on every start, so their definition can change.
const usageSchema = `
	CREATE INDEX IF NOT EXISTS idx_resources_vid_uid ON resources(vid, uid);

	DROP VIEW IF EXISTS volume_usage;
	CREATE VIEW volume_usage AS
	    SELECT v.vid, v.name, v.path, v.dynamic, v.capacity,
	           COALESCE((SELECT SUM(r.size) FROM resources r WHERE r.vid = v.vid), 0) / 1e9 AS usage,
	           v.createdAt
	    FROM volumes v;

	DROP VIEW IF EXISTS user_volume_usage;
	CREATE VIEW user_volume_usage AS
	    SELECT uv.vid, uv.uid,
	           COALESCE((SELECT SUM(r.size) FROM resources r WHERE r.vid = uv.vid AND r.uid = uv.uid), 0) / 1e9 AS usage,
	           uv.quota, uv.updatedAt
	    FROM user_volume uv;
`

// querier is what the checks need from a *sql.DB, *sql.Conn or *sql.Tx.
type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

const gbBytes = 1e9

// checkSpace reports whether size more bytes, owned by uid, fit on the
// named volume: its capacity, and the owner's quota there (the group's on a
// group volume). A user without a quota on the volume gets defaultQuota GB.
// It returns the volume's id.
func checkSpace(ctx context.Context, q querier, uid int64, volume string, size int64, defaultQuota float64) (int64, error) {
	var vid int64
	var capacity float64
	if err := q.QueryRowContext(ctx, `SELECT vid, COALESCE(capacity, 0) FROM volumes WHERE name = ?`, volume).
		Scan(&vid, &capacity); err != nil {
		return 0, fmt.Errorf("volume %q: %w", volume, err)
	}
	var used int64
	if err := q.QueryRowContext(ctx, `SELECT COALESCE(SUM(size), 0) FROM resources WHERE vid = ?`, vid).Scan(&used); err != nil {
		return 0, err
	}
	if capacity > 0 && float64(used+size) > capacity*gbBytes {
		return vid, ErrVolumeFull
	}
	if uid == 0 {
		return vid, nil
	}

	gid, shared, err := groupOf(ctx, q, vid)
	if err != nil {
		return 0, err
	}
	if shared { // the group pays, from everything on the volume
		var quota float64
		if err := q.QueryRowContext(ctx, `SELECT quota FROM group_volume WHERE vid = ?`, vid).Scan(&quota); err != nil {
			return 0, err
		}
		if quota > 0 && float64(used+size) > quota*gbBytes {
			return vid, fmt.Errorf("%w (group %d)", ErrQuotaExceeded, gid)
		}

		return vid, nil
	}

	quota, err := personalQuota(ctx, q, vid, uid, defaultQuota)
	if err != nil {
		return 0, err
	}
	if quota > 0 {
		var mine int64
		if err := q.QueryRowContext(ctx, `SELECT COALESCE(SUM(size), 0) FROM resources WHERE vid = ? AND uid = ?`, vid, uid).
			Scan(&mine); err != nil {
			return 0, err
		}
		if float64(mine+size) > quota*gbBytes {
			return vid, ErrQuotaExceeded
		}
	}

	return vid, nil
}

// personalQuota is uid's quota (GB) on volume vid; a user without one gets
// a claim with defaultQuota.
func personalQuota(ctx context.Context, q querier, vid, uid int64, defaultQuota float64) (float64, error) {
	var quota float64
	err := q.QueryRowContext(ctx, `SELECT COALESCE(quota, 0) FROM user_volume WHERE vid = ? AND uid = ?`, vid, uid).Scan(&quota)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = q.ExecContext(ctx, `INSERT OR IGNORE INTO user_volume (vid, uid, usage, quota, updatedAt) VALUES (?, ?, 0, ?, ?)`,
			vid, uid, defaultQuota, ut.CurrentTime())

		return defaultQuota, err
	}

	return quota, err
}

// CheckSpace reports whether size more bytes owned by uid fit on the named
// volume (ErrVolumeFull / ErrQuotaExceeded). It is a pre-check, to refuse
// before any bytes are written; InsertResource checks again atomically.
func (fsl *FsLite) CheckSpace(ctx context.Context, uid int64, volume string, size int64, defaultQuota float64) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return err
	}
	_, err = checkSpace(ctx, db, uid, volume, size, defaultQuota)

	return err
}

// InsertResource records r, and - with enforce - only if it fits the
// volume's capacity and the owner's (or group's) quota. The check and the
// insert are one transaction that takes SQLite's write lock up front, so
// concurrent inserts can't both squeeze into the same free space.
func (fsl *FsLite) InsertResource(ctx context.Context, r ut.Resource, defaultQuota float64, enforce bool) error {
	return fsl.immediate(ctx, func(q querier) error {
		if r.Vname == "" {
			r.Vname = defaultVolumeName
		}
		r.Name = NormalizeName(r.Name)
		// a taken name is the more useful refusal than a quota
		var taken int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources WHERE vname = ? AND name = ?`, r.Vname, r.Name).Scan(&taken); err != nil {
			return err
		}
		if taken > 0 {
			return fmt.Errorf("%w: %s in %s", ErrResourceExists, r.Name, r.Vname)
		}
		if enforce {
			vid, err := checkSpace(ctx, q, r.UID, r.Vname, r.Size, defaultQuota)
			if err != nil {
				return err
			}
			r.VID = vid
		} else if err := q.QueryRowContext(ctx, `SELECT vid FROM volumes WHERE name = ?`, r.Vname).Scan(&r.VID); err != nil {
			return fmt.Errorf("volume %q: %w", r.Vname, err)
		}

		return insertResource(ctx, q, r)
	})
}

// immediate runs fn in a transaction opened with BEGIN IMMEDIATE (the write
// lock is taken at the start, not at the first write).
func (fsl *FsLite) immediate(ctx context.Context, fn func(q querier) error) (err error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()
	if err := fn(conn); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, "COMMIT")

	return err
}

// SetObjectSize records a new size (and modification time) for an existing
// resource, e.g. when a job overwrote it. Usage follows automatically.
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
