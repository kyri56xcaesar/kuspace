package fslite

/*
	database call handlers for "volumes"
	"userspace.db"

	@used by the api
*/

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

/* database call handlers regarding the Volume table */
func getAllVolumes(ctx context.Context, db *sql.DB) ([]ut.Volume, error) {
	rows, err := db.QueryContext(ctx, `
    SELECT * FROM volume_usage`)
	if err != nil {
		log.Printf("[FSL_DB_getVolumes] error querying db: %v", err)

		return nil, fmt.Errorf("failed to query db: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var volumes []ut.Volume
	for rows.Next() {
		var v ut.Volume
		err = rows.Scan(v.PtrFields()...)
		if err != nil {
			log.Printf("[FSL_DB_getVolumes] error scanning row: %v", err)

			return nil, fmt.Errorf("[fsl] error scanning row: %w", err)
		}

		volumes = append(volumes, v)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return volumes, nil
}

func getVolumeByVid(ctx context.Context, db *sql.DB, vid int) (ut.Volume, error) {
	var volume ut.Volume
	err := db.QueryRowContext(ctx, `SELECT * FROM volume_usage WHERE vid = ?`, vid).Scan(volume.PtrFields()...)
	if errors.Is(err, sql.ErrNoRows) {
		return ut.Volume{}, fmt.Errorf("%w: id %d", ErrVolumeNotFound, vid)
	} else if err != nil {
		log.Printf("[FSL_DB_getVolumeByVid] failed to scan result query: %v", err)

		return ut.Volume{}, fmt.Errorf("[fsl] failed to scan row: %w", err)
	}

	return volume, nil
}

func getVolumeByName(ctx context.Context, db *sql.DB, name string) (ut.Volume, error) {
	var volume ut.Volume
	err := db.QueryRowContext(ctx, `SELECT * FROM volume_usage WHERE name = ?`, name).Scan(volume.PtrFields()...)
	if errors.Is(err, sql.ErrNoRows) {
		return ut.Volume{}, fmt.Errorf("%w: %s", ErrVolumeNotFound, name)
	} else if err != nil {
		log.Printf("failed to scan result query: %v", err)

		return ut.Volume{}, fmt.Errorf("[fsl] failed to scan row: %w", err)
	}

	return volume, nil
}

func deleteVolume(ctx context.Context, db *sql.DB, vid int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[FSL_DB_deleteVolume] failed to begin transaction: %v", err)

		return fmt.Errorf("[fsl] failed to begin transaction %w", err)
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM volumes WHERE vid = ?", vid)
	if err != nil {
		log.Printf("[FSL_DB_deleteVolume] failed to execute delete query: %v", err)

		return fmt.Errorf("[fsl] failed to execute query %w", err)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_deleteVolume] failed to commit transaction: %v", err)

		return fmt.Errorf("[fsl] failed to commit transaction %w", err)
	}

	return nil
}

func deleteVolumeByName(ctx context.Context, db *sql.DB, name string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[FSL_DB_delVolumeByName] failed to begin transaction: %v", err)

		return fmt.Errorf("[fsl] failed to begin transaction %w", err)
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM volumes WHERE name = ?", name)
	if err != nil {
		log.Printf("[FSL_DB_delVolumeByName] failed to execute delete query: %v", err)

		return fmt.Errorf("[fsl] failed to execute query %w", err)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_delVolumeByName] failed to commit transaction: %v", err)

		return fmt.Errorf("[fsl] failed to commit transaction %w", err)
	}

	return nil
}

func insertVolume(ctx context.Context, db *sql.DB, volume ut.Volume) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO 
			volumes (name, path, dynamic, capacity, usage, createdAt) 
		VALUES (?, ?, ?, ?, ?, ?)`, volume.FieldsNoID()...)
	if err != nil {
		log.Printf("[FSL_DB_insertVolume] error upon executing insert query: %v", err)

		return fmt.Errorf("[fsl] failed to execute query %w", err)
	}

	return nil
}

/* database call handlers regarding the UserVolume table */
/* UNIQUE (vid, uid) pair*/
func insertUserVolume(ctx context.Context, db *sql.DB, uv ut.UserVolume) error {
	// check for uniquness
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT 1 FROM user_volume WHERE vid = ? AND uid = ? LIMIT 1;`, uv.VID, uv.UID).Scan(&exists)
	if exists {
		if err == nil {
			return fmt.Errorf("user %d's claim on volume %d %w", uv.UID, uv.VID, ut.ErrExists)
		}
		log.Printf("[FSL_DB_insUv] error checking for uniqunes or not unique: %v", err)

		return fmt.Errorf("error checking for uniqueness or not unique pair: %w", err)
	}

	query := `
		INSERT INTO user_volume (vid, uid, usage, quota, updatedAt)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err = db.ExecContext(ctx, query, uv.VID, uv.UID, uv.Usage, uv.Quota, ut.CurrentTime())
	if err != nil {
		return fmt.Errorf("failed to insert user volume: %w", err)
	}

	return nil
}

func insertUserVolumes(ctx context.Context, db *sql.DB, uvs []ut.UserVolume) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("error starting transaction: %v", err)

		return fmt.Errorf("[fsl] failed to start transaction %w", err)
	}

	placeholder := strings.Repeat("(?, ?, ?, ?, ?),", len(uvs))
	query := "INSERT INTO user_volume (vid, uid, usage, quota, updatedAt) VALUES " + placeholder[:len(placeholder)-1]

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		log.Printf("error preparing transaction: %v", err)

		return fmt.Errorf("[fsl] failed to prepare transaction %w", err)
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()
	for _, uv := range uvs {
		uv.UpdatedAt = ut.CurrentTime()
		_, err = stmt.ExecContext(ctx, uv.Fields()...)
		if err != nil {
			log.Printf("[FSL_DB_insUvs] error executing transaction: %v", err)
			err = tx.Rollback()
			if err != nil {
				return fmt.Errorf("[fsl] failed to execute transaction %w", err)
			}

			return errors.New("failed to exec statement, rolling back")
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_insUvs] failed to commit transaction: %v", err)

		return fmt.Errorf("[fsl] failed to commit transaction %w", err)
	}

	return nil
}

func getAllUserVolumes(ctx context.Context, db *sql.DB) (any, error) {
	query := `SELECT * FROM user_volume_usage`
	rows, err := db.QueryContext(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()
	var userVolumes []ut.UserVolume
	for rows.Next() {
		var uv ut.UserVolume
		err = rows.Scan(uv.PtrFields()...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user volume: %w", err)
		}
		userVolumes = append(userVolumes, uv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return userVolumes, nil
}

func getUserVolumesByUserIDs(ctx context.Context, db *sql.DB, uids []string) (any, error) {
	query := `SELECT * FROM user_volume_usage WHERE uid IN (?` + strings.Repeat(",?", len(uids)-1) + `)`
	if len(uids) == 1 && uids[0] == "*" {
		query = `SELECT * FROM user_volume_usage;`
	}
	args := make([]any, len(uids))
	for i, uid := range uids {
		args[i] = uid
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()
	var userVolumes []ut.UserVolume
	for rows.Next() {
		var uv ut.UserVolume
		err = rows.Scan(uv.PtrFields()...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user volume: %w", err)
		}
		userVolumes = append(userVolumes, uv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return userVolumes, nil
}

func getUserVolumesByVolumeIDs(ctx context.Context, db *sql.DB, vids []string) (any, error) {
	query := `SELECT * FROM user_volume_usage WHERE vid IN (?` + strings.Repeat(",?", len(vids)-1) + `)`
	if len(vids) == 1 && vids[0] == "*" {
		query = `SELECT * FROM user_volume_usage;`
	}
	args := make([]any, len(vids))
	for i, uid := range vids {
		args[i] = uid
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()
	var userVolumes []ut.UserVolume
	for rows.Next() {
		var uv ut.UserVolume
		err = rows.Scan(uv.PtrFields()...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user volume: %w", err)
		}
		userVolumes = append(userVolumes, uv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return userVolumes, nil
}

func getUserVolumesByUidsAndVids(ctx context.Context, db *sql.DB, uids, vids []string) (any, error) {
	query := `SELECT * FROM user_volume_usage
    WHERE 
      vid IN (?` + strings.Repeat(",?", len(vids)-1) + `)
    AND 
      uid IN (?` + strings.Repeat(",?", len(uids)-1) + `)`

	args := make([]any, len(vids))
	for i, uid := range vids {
		args[i] = uid
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()
	var userVolumes []ut.UserVolume
	for rows.Next() {
		var uv ut.UserVolume
		err = rows.Scan(uv.PtrFields()...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user volume: %w", err)
		}
		userVolumes = append(userVolumes, uv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return userVolumes, nil
}
