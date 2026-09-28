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
    SELECT
      *
    FROM 
      volumes`)
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
	err := db.QueryRowContext(ctx, `SELECT * FROM volumes WHERE vid = ?`, vid).Scan(volume.PtrFields()...)
	if err != nil {
		log.Printf("[FSL_DB_getVolumeByVid] failed to scan result query: %v", err)

		return ut.Volume{}, fmt.Errorf("[fsl] failed to scan row: %w", err)
	}

	return volume, nil
}

func getVolumeByName(ctx context.Context, db *sql.DB, name string) (ut.Volume, error) {
	var volume ut.Volume
	err := db.QueryRowContext(ctx, `SELECT * FROM volumes WHERE name = ?`, name).Scan(volume.PtrFields()...)
	if errors.Is(err, sql.ErrNoRows) {
		return ut.Volume{}, errors.New("empty") // or return a custom error
	} else if err != nil {
		log.Printf("failed to scan result query: %v", err)

		return ut.Volume{}, fmt.Errorf("[fsl] failed to scan row: %w", err)
	}

	return volume, nil
}

func updateVolume(ctx context.Context, db *sql.DB, volume ut.Volume) error {
	query := `
		UPDATE 
			volumes
		SET
			name = ?, path = ?, dynamic = ?, capacity = ?, usage = ?
		WHERE
			vid = ?;
	`

	_, err := db.ExecContext(ctx, query, volume.Name, volume.Path, volume.Dynamic, volume.Capacity, volume.Usage, volume.VID)
	if err != nil {
		log.Printf("[FSL_DB_updateVolume] error on query execution: %v", err)

		return fmt.Errorf("[fsl] failed to execute query %w", err)
	}

	return nil
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

func insertVolumes(ctx context.Context, db *sql.DB, volumes []ut.Volume) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[FSL_DB_insertVolumes] error starting transaction: %v", err)

		return fmt.Errorf("[fsl] failed to start transaction %w", err)
	}

	placeholder := strings.Repeat("(?, ?, ?, ?, ?, ?),", len(volumes))
	query := "\n INSERT INTO \n\t\tvolumes (name, path, dynamic, capacity, usage, createdAt) \n    VALUES " + placeholder[:len(placeholder)-1]

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		log.Printf("[FSL_DB_insertVolumes] error preparing transaction: %v", err)

		return fmt.Errorf("[fsl] failed to prepare transaction %w", err)
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()

	for _, v := range volumes {
		_, err = stmt.ExecContext(ctx, v.Path, v.Capacity, v.Usage)
		if err != nil {
			log.Printf("[FSL_DB_insertVolumes] error executing transaction: %v", err)
			err = tx.Rollback()
			if err != nil {
				log.Printf("[FSL_DB_insertVolumes] failed to rollback transaction")

				return fmt.Errorf("[fsl] failed to rollback transaction %w", err)
			}

			return errors.New("[FSL_DB_insertVolumes] failed to execute query, rolling back")
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)

		return fmt.Errorf("[fsl] failed to commit transaction %w", err)
	}

	return nil
}

func deleteVolumeByIDs(ctx context.Context, db *sql.DB, ids []int) error {
	if ids == nil {
		return errors.New("must provide ids")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[FSL_DB_delVolumeByIds] error starting transaction: %v", err)

		return fmt.Errorf("[fsl] failed to start transaction %w", err)
	}

	res, err := tx.ExecContext(ctx, "DELETE FROM volumes WHERE vid IN (?)", ids)
	if err != nil {
		log.Printf("[FSL_DB_delVolumeByIds] failed to exec deletion query: %v", err)

		return fmt.Errorf("[fsl] failed to execute query %w", err)
	}

	_, err = res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_delVolumeByIds] failed to retrieve rows affected: %v", err)

		return fmt.Errorf("[fsl] failed to retrieve rows affected %w", err)
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
			return errors.New("already exists")
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

func deleteUserVolumeByUID(ctx context.Context, db *sql.DB, uid int) error {
	query := `DELETE FROM user_volume WHERE uid = ?`
	_, err := db.ExecContext(ctx, query, uid)
	if err != nil {
		return fmt.Errorf("failed to delete user volume: %w", err)
	}

	return nil
}

func deleteUserVolumeByVid(ctx context.Context, db *sql.DB, vid int) error {
	_, err := db.ExecContext(ctx, "DELETE FROM user_volume WHERE vid = ?", vid)
	if err != nil {
		return fmt.Errorf("failed to delete user volume: %w", err)
	}

	return nil
}

func updateUserVolume(ctx context.Context, db *sql.DB, uv ut.UserVolume) error {
	query := `
		UPDATE user_volume
		SET usage = ?, quota = ?, updatedAt = ?
		WHERE vid = ? AND uid = ?
	`
	_, err := db.ExecContext(ctx, query, uv.Usage, uv.Quota, ut.CurrentTime(), uv.VID, uv.UID)
	if err != nil {
		return fmt.Errorf("failed to update user volume: %w", err)
	}

	return nil
}

func updateUserVolumeQuotaByUID(ctx context.Context, db *sql.DB, quota float32, uid int) error {
	query := `UPDATE user_volume SET quota = ? WHERE uid = ?`
	res, err := db.ExecContext(ctx, query, quota, uid)
	if err != nil {
		log.Printf("[FSL_DB_updateUvQuotaByUid] failed to exec query: %v", err)

		return fmt.Errorf("[fsl] failed to execute query %w", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateUvQuotaByUid] failed to retrieve info about rows affected")

		return fmt.Errorf("[fsl] failed to retrieve rows affected %w", err)
	}

	return nil
}

func updateUserVolumeUsageByUID(ctx context.Context, db *sql.DB, usage float32, uid int) error {
	query := `UPDATE user_volume SET usage = ? WHERE uid = ?`
	res, err := db.ExecContext(ctx, query, usage, uid)
	if err != nil {
		log.Printf("[FSL_DB_updateUvUsageByUid] failed to exec query: %v", err)

		return fmt.Errorf("[fsl] failed to execute query %w", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateUvUsageByUid] failed to retrieve info about rows affected")

		return fmt.Errorf("[fsl] failed to retrieve rows affected %w", err)
	}

	return nil
}

func updateUserVolumeQuotaAndUsageByUID(ctx context.Context, db *sql.DB, usage, quota float32, uid int) error {
	query := `UPDATE user_volume SET usage = ?, quota = ? WHERE uid = ?`
	res, err := db.ExecContext(ctx, query, usage, quota, uid)
	if err != nil {
		log.Printf("[FSL_DB_updateUvQuotaUsageByUid] failed to exec query: %v", err)

		return fmt.Errorf("[fsl] failed to execute query %w", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateUvQuotaUsageByUid] failed to retrieve info about rows affected")

		return fmt.Errorf("[fsl] failed to retrieve rows affected %w", err)
	}

	return nil
}

func getAllUserVolumes(ctx context.Context, db *sql.DB) (any, error) {
	query := `SELECT * FROM user_volume`
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

func getUserVolumeByUID(ctx context.Context, db *sql.DB, uid int64) (ut.UserVolume, error) {
	query := `SELECT * FROM user_volume WHERE uid = ?`
	var userVolume ut.UserVolume
	err := db.QueryRowContext(ctx, query, uid).Scan(userVolume.PtrFields()...)
	if err != nil {
		return ut.UserVolume{}, fmt.Errorf("failed to query user volume: %w", err)
	}

	return userVolume, nil
}

func getUserVolumesByUserIDs(ctx context.Context, db *sql.DB, uids []string) (any, error) {
	query := `SELECT * FROM user_volume WHERE uid IN (?` + strings.Repeat(",?", len(uids)-1) + `)`
	if len(uids) == 1 && uids[0] == "*" {
		query = `SELECT * FROM user_volume;`
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
	query := `SELECT * FROM user_volume WHERE vid IN (?` + strings.Repeat(",?", len(vids)-1) + `)`
	if len(vids) == 1 && vids[0] == "*" {
		query = `SELECT * FROM user_volume;`
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
	query := `SELECT * FROM 
      userVolume 
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

/* database call handlers regarding the GroupVolume tabke*/

/* UNIQUE pair (gid, vid) SHOULD BE (we checking)*/
func insertGroupVolume(ctx context.Context, db *sql.DB, gv ut.GroupVolume) error {
	// check for uniquness
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT 1 FROM groupVolume WHERE vid = ? AND gid = ? LIMIT 1;`, gv.VID, gv.GID).Scan(&exists)
	if exists {
		if err == nil {
			return errors.New("already exists")
		}
		log.Printf("error checking for uniqunes or not unique: %v", err)

		return fmt.Errorf("error checking for uniqueness or not unique pair: %w", err)
	}

	query := `
		INSERT INTO groupVolume (vid, gid, usage, quota, updatedAt)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err = db.ExecContext(ctx, query, gv.VID, gv.GID, gv.Usage, gv.Quota, ut.CurrentTime())
	if err != nil {
		return fmt.Errorf("failed to insert group volume: %w", err)
	}

	return nil
}

func insertGroupVolumes(ctx context.Context, db *sql.DB, gvs []ut.GroupVolume) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[FSL_DB_insGvs] error starting transaction: %v", err)

		return fmt.Errorf("[fsl] failed to start transaction %w", err)
	}
	placeholder := strings.Repeat("(?, ?, ?, ?, ?),", len(gvs))
	query := "INSERT INTO  groupVolume (vid, gid, usage, quota, updatedAt) VALUES " + placeholder[:len(placeholder)-1]

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		log.Printf("[FSL_DB_insGvs] error preparing transaction: %v", err)

		return fmt.Errorf("[fsl] failed to prepare transaction %w", err)
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()
	for _, gv := range gvs {
		gv.UpdatedAt = ut.CurrentTime()
		_, err = stmt.ExecContext(ctx, gv.Fields()...)
		if err != nil {
			log.Printf("[FSL_DB_insGvs] error executing transaction: %v", err)
			err = tx.Rollback()
			if err != nil {
				return fmt.Errorf("[fsl] failed to rollback transaction %w", err)
			}

			return errors.New("failed to exec statement rolling back")
		}
	}
	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_insGvs] failed to commit transaction: %v", err)

		return fmt.Errorf("[fsl] failed to commit transaction %w", err)
	}

	return nil
}

func deleteGroupVolumeByGID(ctx context.Context, db *sql.DB, gid int) error {
	_, err := db.ExecContext(ctx, "DELETE FROM groupVolume WHERE gid = ?", gid)
	if err != nil {
		return fmt.Errorf("failed to delete group volume: %w", err)
	}

	return nil
}

func deleteGroupVolumeByVid(ctx context.Context, db *sql.DB, vid int) error {
	_, err := db.ExecContext(ctx, "DELETE FROM groupVolume WHERE vid = ?", vid)
	if err != nil {
		return fmt.Errorf("failed to delete group volume: %w", err)
	}

	return nil
}

func updateGroupVolume(ctx context.Context, db *sql.DB, gv ut.GroupVolume) error {
	query := `
		UPDATE groupVolume
		SET usage = ?, quota = ?, updatedAt = ?
		WHERE vid = ? AND gid = ?
	`
	_, err := db.ExecContext(ctx, query, gv.Usage, gv.Quota, ut.CurrentTime(), gv.VID, gv.GID)
	if err != nil {
		return fmt.Errorf("failed to update group volume: %w", err)
	}

	return nil
}

func updateGroupVolumes(ctx context.Context, db *sql.DB, gvs []ut.GroupVolume) error {
	if len(gvs) == 0 {
		return nil // No updates needed
	}
	query := `
		UPDATE groupVolume
		SET usage = ?, quota = ?, updatedAt = ?
		WHERE vid = ? AND gid = ?
	`
	// Begin a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		err = tx.Rollback()
		if err != nil {
			return fmt.Errorf("failed to prepare transaction: %w", err)
		}

		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()
	for _, gv := range gvs {
		_, err := stmt.ExecContext(ctx, gv.Usage, gv.Quota, ut.CurrentTime(), gv.VID, gv.GID)
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				return fmt.Errorf("failed to rollback transaction: %w", err)
			}

			return fmt.Errorf("failed to update group volume (vid=%d, gid=%d): %w", gv.VID, gv.GID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func updateGroupVolumesUsageByGids(ctx context.Context, db *sql.DB, gids []string) error {
	query := `
    UPDATE groupVolume 
    SET usage = ? 
    WHERE gid IN (?
    ` + strings.Repeat(", ?", len(gids)-1) + `)`

	args := make([]any, len(gids))
	for i, v := range gids {
		args[i] = v
	}
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		log.Printf("[FSL_DB_updateGvUsageByGids] failed to exec query: %v", err)

		return fmt.Errorf("failed to execute query: %w", err)
	}

	_, err = res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateGvUsageByGids] failed to retrieve rows affected: %v", err)

		return fmt.Errorf("failed to begin retrieve rows affected: %w", err)
	}

	return nil
}

func updateGroupVolumeQuotaByGID(ctx context.Context, db *sql.DB, quota float32, gid int) error {
	query := `
    UPDATE 
      groupVolume
    SET
      quota = ? 
    WHERE 
      gid = ?
      
  `
	res, err := db.ExecContext(ctx, query, quota, gid)
	if err != nil {
		log.Printf("[FSL_DB_updateGvQuotaByGid] failed to exec query: %v", err)

		return fmt.Errorf("failed to execute query: %w", err)
	}

	_, err = res.RowsAffected()
	if err != nil {
		log.Printf("failed to retrieve info about rows affected")

		return fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	return nil
}

func updateGroupVolumeUsageByGID(ctx context.Context, db *sql.DB, usage float32, gid int) error {
	query := `
    UPDATE 
      groupVolume
    SET
      usage = ? 
    WHERE 
      gid = ?
      
  `
	res, err := db.ExecContext(ctx, query, usage, gid)
	if err != nil {
		log.Printf("[FSL_DB_updateGvUsageByGid] failed to exec query: %v", err)

		return fmt.Errorf("failed to execute query: %w", err)
	}

	_, err = res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateGvUsageByGid] failed to retrieve info about rows affected")

		return fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	return nil
}

func updateGroupVolumeQuotaAndUsageByUID(ctx context.Context, db *sql.DB, usage, quota float32, gid int) error {
	query := `
    UPDATE 
      groupVolume
    SET
      usage = ?, quota = ? 
    WHERE 
      gid = ?
      
  `
	res, err := db.ExecContext(ctx, query, usage, quota, gid)
	if err != nil {
		log.Printf("[FSL_DB_updateGvUsageQuotaByGid] failed to exec query: %v", err)

		return fmt.Errorf("failed to execute query: %w", err)
	}

	_, err = res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateGvUsageQuotaByGid] failed to retrieve info about rows affected")

		return fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	return nil
}

func getAllGroupVolumes(ctx context.Context, db *sql.DB) (any, error) {
	query := `SELECT * FROM groupVolume`
	rows, err := db.QueryContext(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()
	var groupVolumes []ut.GroupVolume
	for rows.Next() {
		var gv ut.GroupVolume
		err = rows.Scan(gv.PtrFields()...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan group volume: %w", err)
		}
		groupVolumes = append(groupVolumes, gv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return groupVolumes, nil
}

func getGroupVolumeByGID(ctx context.Context, db *sql.DB, gid int) (ut.GroupVolume, error) {
	query := `SELECT * FROM groupVolume WHERE gid = ?`
	var groupVolume ut.GroupVolume
	err := db.QueryRowContext(ctx, query, gid).Scan(groupVolume.PtrFields()...)
	if err != nil {
		return ut.GroupVolume{}, fmt.Errorf("failed to query group volumes: %w", err)
	}

	return groupVolume, nil
}

func getGroupVolumesByGroupIDs(ctx context.Context, db *sql.DB, gids []string) (any, error) {
	query := `SELECT * FROM groupVolume WHERE gid IN (?` + strings.Repeat(",?", len(gids)-1) + `)`
	if len(gids) == 1 && gids[0] == "*" {
		query = `SELECT * FROM groupVolume;`
	}
	args := make([]any, len(gids))
	for i, uid := range gids {
		args[i] = uid
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()
	var groupVolumes []ut.GroupVolume
	for rows.Next() {
		var gv ut.GroupVolume
		err = rows.Scan(gv.PtrFields()...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan group volume: %w", err)
		}
		groupVolumes = append(groupVolumes, gv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return groupVolumes, nil
}

func getGroupVolumesByVolumeIDs(ctx context.Context, db *sql.DB, vids []string) (any, error) {
	query := `SELECT * FROM groupVolume WHERE vid IN (?` + strings.Repeat(",?", len(vids)-1) + `)`
	if len(vids) == 1 && vids[0] == "*" {
		query = `SELECT * FROM groupVolume;`
	}
	args := make([]any, len(vids))
	for i, uid := range vids {
		args[i] = uid
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()
	var groupVolumes []ut.GroupVolume
	for rows.Next() {
		var gv ut.GroupVolume
		err = rows.Scan(gv.PtrFields()...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan group volume: %w", err)
		}
		groupVolumes = append(groupVolumes, gv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return groupVolumes, nil
}

func getGroupVolumesByVidsAndGids(ctx context.Context, db *sql.DB, vids, gids []string) (any, error) {
	query := `SELECT * FROM 
      groupVolume 
    WHERE 
      vid IN (?` + strings.Repeat(",?", len(vids)-1) + `)
    AND 
      gid IN (?` + strings.Repeat(",?", len(gids)-1) + `)
	`

	args := make([]any, len(vids))
	for i, uid := range vids {
		args[i] = uid
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var groupVolumes []ut.GroupVolume
	for rows.Next() {
		var gv ut.GroupVolume
		err = rows.Scan(gv.PtrFields()...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan group volume: %w", err)
		}
		groupVolumes = append(groupVolumes, gv)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return groupVolumes, nil
}
