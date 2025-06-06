package fslite

/*
	database call handlers for "volumes"
	"userspace.db"

	@used by the api
*/

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

/* database call handlers regarding the Volume table */
func getAllVolumes(db *sql.DB) ([]ut.Volume, error) {
	rows, err := db.Query(`
    SELECT
      *
    FROM 
      volumes`)
	if err != nil {
		log.Printf("[FSL_DB_getVolumes] error querying db: %v", err)
		return nil, err
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
			return nil, err
		}

		volumes = append(volumes, v)
	}

	return volumes, nil
}

func getVolumeByVid(db *sql.DB, vid int) (ut.Volume, error) {
	var volume ut.Volume
	err := db.QueryRow(`SELECT * FROM volumes WHERE vid = ?`, vid).Scan(volume.PtrFields()...)
	if err != nil {
		log.Printf("[FSL_DB_getVolumeByVid] failed to scan result query: %v", err)
		return ut.Volume{}, err
	}
	return volume, nil
}

func getVolumeByName(db *sql.DB, name string) (ut.Volume, error) {
	var volume ut.Volume
	err := db.QueryRow(`SELECT * FROM volumes WHERE name = ?`, name).Scan(volume.PtrFields()...)
	if errors.Is(err, sql.ErrNoRows) {
		return ut.Volume{}, errors.New("empty") // or return a custom error
	} else if err != nil {
		log.Printf("failed to scan result query: %v", err)
		return ut.Volume{}, err
	}
	return volume, nil
}

func updateVolume(db *sql.DB, volume ut.Volume) error {
	query := `
		UPDATE 
			volumes
		SET
			name = ?, path = ?, dynamic = ?, capacity = ?, usage = ?
		WHERE
			vid = ?;
	`

	_, err := db.Exec(query, volume.Name, volume.Path, volume.Dynamic, volume.Capacity, volume.Usage, volume.Vid)
	if err != nil {
		log.Printf("[FSL_DB_updateVolume] error on query execution: %v", err)
		return err
	}

	return nil
}

func deleteVolume(db *sql.DB, vid int) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_deleteVolume] failed to begin transaction: %v", err)
		return err
	}
	_, err = tx.Exec("DELETE FROM volumes WHERE vid = ?", vid)
	if err != nil {
		log.Printf("[FSL_DB_deleteVolume] failed to execute delete query: %v", err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_deleteVolume] failed to commit transaction: %v", err)
		return err
	}
	return nil
}

func deleteVolumeByName(db *sql.DB, name string) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_delVolumeByName] failed to begin transaction: %v", err)
		return err
	}
	_, err = tx.Exec("DELETE FROM volumes WHERE name = ?", name)
	if err != nil {
		log.Printf("[FSL_DB_delVolumeByName] failed to execute delete query: %v", err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_delVolumeByName] failed to commit transaction: %v", err)
		return err
	}
	return nil
}

func insertVolume(db *sql.DB, volume ut.Volume) error {
	_, err := db.Exec(`
		INSERT INTO 
			volumes (vid, name, path, dynamic, capacity, usage, createdAt) 
		VALUES (nextval('seq_volumeid'), ?, ?, ?, ?, ?, ?)`, volume.FieldsNoID()...)
	if err != nil {
		log.Printf("[FSL_DB_insertVolume] error upon executing insert query: %v", err)
		return err
	}
	return nil
}

func insertVolumes(db *sql.DB, volumes []ut.Volume) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_insertVolumes] error starting transaction: %v", err)
		return err
	}

	placeholder := strings.Repeat("(nextval('seq_volumeid'), ?, ?, ?, ?, ?, ?),", len(volumes))
	query := fmt.Sprintf(`
    INSERT INTO 
		volumes (vid, name, path, dynamic, capacity, usage, createdAt) 
    VALUES %s`, placeholder[:len(placeholder)-1])

	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("[FSL_DB_insertVolumes] error preparing transaction: %v", err)
		return err
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()

	for _, v := range volumes {
		_, err = stmt.Exec(v.Path, v.Capacity, v.Usage)
		if err != nil {
			log.Printf("[FSL_DB_insertVolumes] error executing transaction: %v", err)
			err = tx.Rollback()
			if err != nil {
				return err
			}
			return fmt.Errorf("[FSL_DB_insertVolumes] failed to exec query, rolling back")
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func deleteVolumeByIDs(db *sql.DB, ids []int) error {
	if ids == nil {
		return fmt.Errorf("must provide ids")
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_delVolumeByIds] error starting transaction: %v", err)
		return err
	}

	res, err := tx.Exec("DELETE FROM volumes WHERE vid IN (?)", ids)
	if err != nil {
		log.Printf("[FSL_DB_delVolumeByIds] failed to exec deleteion query: %v", err)
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_delVolumeByIds] failed to retrieve rows affected: %v", err)
		return err
	}

	if verbose {
		log.Printf("[FSL_DB_delVolumeByIds] deleted %v entries", rowsAffected)
	}

	return nil
}

/* database call handlers regarding the UserVolume table */
/* UNIQUE (vid, uid) pair*/
func insertUserVolume(db *sql.DB, uv ut.UserVolume) error {
	// check for uniquness
	var exists bool
	err := db.QueryRow(`SELECT 1 FROM userVolume WHERE vid = ? AND uid = ? LIMIT 1;`, uv.Vid, uv.UID).Scan(&exists)
	if exists {
		if err == nil {
			return fmt.Errorf("already exists")
		}
		log.Printf("[FSL_DB_insUv] error checking for uniqunes or not unique: %v", err)
		return fmt.Errorf("error checking for uniqueness or not unique pair: %v", err)
	}

	query := `
		INSERT INTO userVolume (vid, uid, usage, quota, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")

	_, err = db.Exec(query, uv.Vid, uv.UID, uv.Usage, uv.Quota, currentTime)
	if err != nil {
		return fmt.Errorf("failed to insert user volume: %v", err)
	}

	return nil
}

func insertUserVolumes(db *sql.DB, uvs []ut.UserVolume) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("error starting transaction: %v", err)
		return err
	}

	placeholder := strings.Repeat("(?, ?, ?, ?, ?),", len(uvs))
	query := fmt.Sprintf(`
    INSERT INTO 
      userVolume (vid, uid, usage, quota, updated_at)
    VALUES %s`, placeholder[:len(placeholder)-1])

	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("error preparing transaction: %v", err)
		return err
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()
	for _, uv := range uvs {
		uv.UpdatedAt = ut.CurrentTime()
		_, err = stmt.Exec(uv.Fields()...)
		if err != nil {
			log.Printf("[FSL_DB_insUvs] error executing transaction: %v", err)
			err = tx.Rollback()
			if err != nil {
				return err
			}
			return fmt.Errorf("failed to exec statement, rolling back")
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_insUvs] failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func deleteUserVolumeByUID(db *sql.DB, uid int) error {
	query := `DELETE FROM userVolume WHERE uid = ?`

	_, err := db.Exec(query, uid)
	if err != nil {
		return fmt.Errorf("failed to delete user volume: %v", err)
	}

	return nil
}

func deleteUserVolumeByVid(db *sql.DB, vid int) error {
	query := `DELETE FROM userVolume WHERE vid = ?`

	_, err := db.Exec(query, vid)
	if err != nil {
		return fmt.Errorf("failed to delete user volume: %v", err)
	}

	return nil
}

func updateUserVolume(db *sql.DB, uv ut.UserVolume) error {
	query := `
		UPDATE userVolume
		SET usage = ?, quota = ?, updated_at = ?
		WHERE vid = ? AND uid = ?
	`
	_, err := db.Exec(query, uv.Usage, uv.Quota, ut.CurrentTime(), uv.Vid, uv.UID)
	if err != nil {
		return fmt.Errorf("failed to update user volume: %v", err)
	}

	return nil
}

func updateUserVolumeQuotaByUID(db *sql.DB, quota float32, uid int) error {
	query := `UPDATE userVolume SET quota = ? WHERE uid = ?`
	res, err := db.Exec(query, quota, uid)
	if err != nil {
		log.Printf("[FSL_DB_updateUvQuotaByUid] failed to exec query: %v", err)
		return err
	}
	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateUvQuotaByUid] failed to retrieve info about rows affected")
		return err
	}
	if verbose {
		log.Printf("[FSL_DB_updateUvQuotaByUid] rows affected: %v", rAff)
	}
	return nil
}

func updateUserVolumeUsageByUID(db *sql.DB, usage float32, uid int) error {
	query := `UPDATE userVolume SET usage = ? WHERE uid = ?`
	res, err := db.Exec(query, usage, uid)
	if err != nil {
		log.Printf("[FSL_DB_updateUvUsageByUid] failed to exec query: %v", err)
		return err
	}
	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateUvUsageByUid] failed to retrieve info about rows affected")
		return err
	}
	if verbose {
		log.Printf("[FSL_DB_updateUvUsageByUid] rows affected: %v", rAff)
	}
	return nil
}

func updateUserVolumeQuotaAndUsageByUID(db *sql.DB, usage, quota float32, uid int) error {
	query := `UPDATE userVolume SET usage = ?, quota = ? WHERE uid = ?`
	res, err := db.Exec(query, usage, quota, uid)
	if err != nil {
		log.Printf("[FSL_DB_updateUvQuotaUsageByUid] failed to exec query: %v", err)
		return err
	}
	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateUvQuotaUsageByUid] failed to retrieve info about rows affected")
		return err
	}
	if verbose {
		log.Printf("[FSL_DB_updateUvQuotaUsageByUid] rows affected: %v", rAff)
	}
	return nil
}

func getAllUserVolumes(db *sql.DB) (any, error) {
	query := `SELECT * FROM userVolume`
	rows, err := db.Query(query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %v", err)
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
			return nil, fmt.Errorf("failed to scan user volume: %v", err)
		}
		userVolumes = append(userVolumes, uv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}
	return userVolumes, nil
}

func getUserVolumeByUID(db *sql.DB, uid int) (ut.UserVolume, error) {
	query := `SELECT * FROM userVolume WHERE uid = ?`
	var userVolume ut.UserVolume
	err := db.QueryRow(query, uid).Scan(userVolume.PtrFields()...)
	if err != nil {
		return ut.UserVolume{}, fmt.Errorf("failed to query user volume: %v", err)
	}
	return userVolume, nil
}

func getUserVolumesByUserIDs(db *sql.DB, uids []string) (any, error) {
	query := `SELECT * FROM userVolume WHERE uid IN (?` + strings.Repeat(",?", len(uids)-1) + `)`
	if len(uids) == 1 && uids[0] == "*" {
		query = `SELECT * FROM userVolume;`
	}
	args := make([]any, len(uids))
	for i, uid := range uids {
		args[i] = uid
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %v", err)
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
			return nil, fmt.Errorf("failed to scan user volume: %v", err)
		}
		userVolumes = append(userVolumes, uv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}
	return userVolumes, nil
}

func getUserVolumesByVolumeIDs(db *sql.DB, vids []string) (any, error) {
	query := `SELECT * FROM userVolume WHERE vid IN (?` + strings.Repeat(",?", len(vids)-1) + `)`
	if len(vids) == 1 && vids[0] == "*" {
		query = `SELECT * FROM userVolume;`
	}
	args := make([]any, len(vids))
	for i, uid := range vids {
		args[i] = uid
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %v", err)
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
			return nil, fmt.Errorf("failed to scan user volume: %v", err)
		}
		userVolumes = append(userVolumes, uv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}
	return userVolumes, nil
}

func getUserVolumesByUidsAndVids(db *sql.DB, uids, vids []string) (any, error) {
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
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %v", err)
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
			return nil, fmt.Errorf("failed to scan user volume: %v", err)
		}
		userVolumes = append(userVolumes, uv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}
	return userVolumes, nil
}

/* database call handlers regarding the GroupVolume tabke*/

/* UNIQUE pair (gid, vid) SHOULD BE (we checking)*/
func insertGroupVolume(db *sql.DB, gv ut.GroupVolume) error {
	// check for uniquness
	var exists bool
	err := db.QueryRow(`SELECT 1 FROM groupVolume WHERE vid = ? AND gid = ? LIMIT 1;`, gv.Vid, gv.Gid).Scan(&exists)
	if exists {
		if err == nil {
			return fmt.Errorf("already exists")
		}
		log.Printf("error checking for uniqunes or not unique: %v", err)
		return fmt.Errorf("error checking for uniqueness or not unique pair: %v", err)
	}

	query := `
		INSERT INTO groupVolume (vid, gid, usage, quota, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err = db.Exec(query, gv.Vid, gv.Gid, gv.Usage, gv.Quota, ut.CurrentTime())
	if err != nil {
		return fmt.Errorf("failed to insert group volume: %v", err)
	}

	return nil
}

func insertGroupVolumes(db *sql.DB, gvs []ut.GroupVolume) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_insGvs] error starting transaction: %v", err)
		return err
	}
	placeholder := strings.Repeat("(?, ?, ?, ?, ?),", len(gvs))
	query := fmt.Sprintf(`
    INSERT INTO 
      groupVolume (vid, gid, usage, quota, updated_at)
    VALUES %s`, placeholder[:len(placeholder)-1])

	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("[FSL_DB_insGvs] error preparing transaction: %v", err)
		return err
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()
	for _, gv := range gvs {
		gv.UpdatedAt = ut.CurrentTime()
		_, err = stmt.Exec(gv.Fields()...)
		if err != nil {
			log.Printf("[FSL_DB_insGvs] error executing transaction: %v", err)
			err = tx.Rollback()
			if err != nil {
				return err
			}
			return fmt.Errorf("failed to exec statement rolling back")
		}
	}
	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_insGvs] failed to commit transaction: %v", err)
		return err
	}
	return nil
}

func deleteGroupVolumeByGid(db *sql.DB, gid int) error {
	query := `DELETE FROM groupVolume WHERE gid = ?`

	_, err := db.Exec(query, gid)
	if err != nil {
		return fmt.Errorf("failed to delete group volume: %v", err)
	}

	return nil
}

func deleteGroupVolumeByVid(db *sql.DB, vid int) error {
	query := `DELETE FROM groupVolume WHERE vid = ?`
	_, err := db.Exec(query, vid)
	if err != nil {
		return fmt.Errorf("failed to delete group volume: %v", err)
	}
	return nil
}

func updateGroupVolume(db *sql.DB, gv ut.GroupVolume) error {
	query := `
		UPDATE groupVolume
		SET usage = ?, quota = ?, updated_at = ?
		WHERE vid = ? AND gid = ?
	`
	_, err := db.Exec(query, gv.Usage, gv.Quota, ut.CurrentTime(), gv.Vid, gv.Gid)
	if err != nil {
		return fmt.Errorf("failed to update group volume: %v", err)
	}

	return nil
}

func updateGroupVolumes(db *sql.DB, gvs []ut.GroupVolume) error {
	if len(gvs) == 0 {
		return nil // No updates needed
	}
	query := `
		UPDATE groupVolume
		SET usage = ?, quota = ?, updated_at = ?
		WHERE vid = ? AND gid = ?
	`
	// Begin a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	stmt, err := tx.Prepare(query)
	if err != nil {
		err = tx.Rollback()
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()
	for _, gv := range gvs {
		_, err := stmt.Exec(gv.Usage, gv.Quota, ut.CurrentTime(), gv.Vid, gv.Gid)
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				return err
			}
			return fmt.Errorf("failed to update group volume (vid=%d, gid=%d): %v", gv.Vid, gv.Gid, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func updateGroupVolumesUsageByGids(db *sql.DB, gids []string) error {
	query := `
    UPDATE groupVolume 
    SET usage = ? 
    WHERE gid IN (?
    ` + strings.Repeat(", ?", len(gids)-1) + `)`

	args := make([]any, len(gids))
	for i, v := range gids {
		args[i] = v
	}
	res, err := db.Exec(query, args...)
	if err != nil {
		log.Printf("[FSL_DB_updateGvUsageByGids] failed to exec query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateGvUsageByGids] failed to retrieve rows affected: %v", err)
		return err
	}
	if verbose {
		log.Printf("[FSL_DB_updateGvUsageByGids] len(gids): %v, rAff: %v", len(gids), rAff)
	}
	return nil
}

func updateGroupVolumeQuotaByGid(db *sql.DB, quota float32, gid int) error {
	query := `
    UPDATE 
      groupVolume
    SET
      quota = ? 
    WHERE 
      gid = ?
      
  `
	res, err := db.Exec(query, quota, gid)
	if err != nil {
		log.Printf("[FSL_DB_updateGvQuotaByGid] failed to exec query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to retrieve info about rows affected")
		return err
	}
	if verbose {
		log.Printf("[FSL_DB_updateGvQuotaByGid] rows affected: %v", rAff)
	}
	return nil
}

func updateGroupVolumeUsageByGid(db *sql.DB, usage float32, gid int) error {
	query := `
    UPDATE 
      groupVolume
    SET
      usage = ? 
    WHERE 
      gid = ?
      
  `
	res, err := db.Exec(query, usage, gid)
	if err != nil {
		log.Printf("[FSL_DB_updateGvUsageByGid] failed to exec query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateGvUsageByGid] failed to retrieve info about rows affected")
		return err
	}
	if verbose {
		log.Printf("[FSL_DB_updateGvUsageByGid] rows affected: %v", rAff)
	}
	return nil
}

func updateGroupVolumeQuotaAndUsageByUID(db *sql.DB, usage, quota float32, gid int) error {
	query := `
    UPDATE 
      groupVolume
    SET
      usage = ?, quota = ? 
    WHERE 
      gid = ?
      
  `
	res, err := db.Exec(query, usage, quota, gid)
	if err != nil {
		log.Printf("[FSL_DB_updateGvUsageQuotaByGid] failed to exec query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateGvUsageQuotaByGid] failed to retrieve info about rows affected")
		return err
	}
	if verbose {
		log.Printf("[FSL_DB_updateGvUsageQuotaByGid] rows affected: %v", rAff)
	}
	return nil
}

func getAllGroupVolumes(db *sql.DB) (any, error) {
	query := `SELECT * FROM groupVolume`
	rows, err := db.Query(query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %v", err)
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
			return nil, fmt.Errorf("failed to scan group volume: %v", err)
		}
		groupVolumes = append(groupVolumes, gv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}
	return groupVolumes, nil
}

func getGroupVolumeByGid(db *sql.DB, gid int) (ut.GroupVolume, error) {
	query := `SELECT * FROM groupVolume WHERE gid = ?`
	var groupVolume ut.GroupVolume
	err := db.QueryRow(query, gid).Scan(groupVolume.PtrFields()...)
	if err != nil {
		return ut.GroupVolume{}, fmt.Errorf("failed to query group volumes: %v", err)
	}
	return groupVolume, nil
}

func getGroupVolumesByGroupIDs(db *sql.DB, gids []string) (any, error) {
	query := `SELECT * FROM groupVolume WHERE gid IN (?` + strings.Repeat(",?", len(gids)-1) + `)`
	if len(gids) == 1 && gids[0] == "*" {
		query = `SELECT * FROM groupVolume;`
	}
	args := make([]any, len(gids))
	for i, uid := range gids {
		args[i] = uid
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %v", err)
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
			return nil, fmt.Errorf("failed to scan group volume: %v", err)
		}
		groupVolumes = append(groupVolumes, gv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}
	return groupVolumes, nil
}

func getGroupVolumesByVolumeIDs(db *sql.DB, vids []string) (any, error) {
	query := `SELECT * FROM groupVolume WHERE vid IN (?` + strings.Repeat(",?", len(vids)-1) + `)`
	if len(vids) == 1 && vids[0] == "*" {
		query = `SELECT * FROM groupVolume;`
	}
	args := make([]any, len(vids))
	for i, uid := range vids {
		args[i] = uid
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %v", err)
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
			return nil, fmt.Errorf("failed to scan group volume: %v", err)
		}
		groupVolumes = append(groupVolumes, gv)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}
	return groupVolumes, nil
}

func getGroupVolumesByVidsAndGids(db *sql.DB, vids, gids []string) (any, error) {
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

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %v", err)
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
			return nil, fmt.Errorf("failed to scan group volume: %v", err)
		}
		groupVolumes = append(groupVolumes, gv)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}

	return groupVolumes, nil
}

func initResourceVolumeSpecific(db *sql.DB, volumesPath, capacity string) {
	// specific initialization
	var (
		exists bool
		vid    int64
	)
	err := db.QueryRow("SELECT COUNT(*) > 0 FROM volumes").Scan(&exists)
	if err != nil {
		log.Fatalf("failed to scan exists query: %v", err)
	}
	if !exists {
		// insert an init volume
		vquery := `
		INSERT INTO 
			volumes (vid, name, path, dynamic, capacity, usage)
		VALUES
			(nextval('seq_volumeid'), ?, ?, ?, ?, ?)
		RETURNING (vid);
		 `
		err := db.QueryRow(vquery, defaultVolumeName, volumesPath, "true", capacity, 0).Scan(&vid)
		if err != nil {
			log.Fatalf("failed to insert init data, destructive: %v", err)
		}
	}

	// insert root user/group volume claims(should be everything...)
	// or perhaps avoid it? idk
	err = db.QueryRow("SELECT COUNT(*) > 0 FROM userVolume").Scan(&exists)
	if err != nil {
		log.Fatalf("failed to scan exists query: %v", err)
	}

	if !exists {
		// insert an init volume
		vquery := `
		INSERT INTO 
			userVolume (vid, uid, usage, quota, updated_at)
		VALUES
			(?, ?, ?, ?, ?)
		 `

		currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")

		_, err = db.Exec(vquery, vid, 0, 0, capacity, currentTime)
		if err != nil {
			log.Fatalf("failed to insert init data, destructive: %v", err)
		}
	}

	err = db.QueryRow("SELECT COUNT(*) > 0 FROM groupVolume").Scan(&exists)
	if err != nil {
		log.Fatalf("failed to scan exists query: %v", err)
	}

	if !exists {
		// insert an init volume
		vquery := `
		INSERT INTO 
			groupVolume (vid, gid, usage, quota, updated_at)
		VALUES
			(?, ?, ?, ?, ?),
    	(?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?)

		 `

		currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")

		res, err := db.Exec(vquery, vid, 0, 0, capacity, currentTime, vid, 100, 0, capacity, currentTime, vid, 1000, 0, capacity, currentTime)
		if err != nil {
			log.Fatalf("failed to insert init data, destructive: %v", err)
		}

		rAff, err := res.RowsAffected()
		if err != nil {
			log.Printf("failed to retrieve rows affected: %v", err)
			return
		}

		if rAff != 3 {
			log.Fatalf("failed to insert essential groupVolumes")
		}
	}
}

// updated, better, more inclusive funcs
func getVolumes(db *sql.DB, sel, table, by, byvalue string, limit int) ([]any, error) {
	if sel == "" {
		sel = "*"
	}
	query := fmt.Sprintf("SELECT %s FROM %s", sel, table)

	var placeholderStr string
	var args []any
	for _, arg := range strings.Split(strings.TrimSpace(byvalue), ",") {
		args = append(args, arg)
	}
	if by != "" && byvalue != "" {
		byValuesSplit := strings.Split(strings.TrimSpace(byvalue), ",")
		placeholders := make([]string, len(byValuesSplit))
		for i := range byValuesSplit {
			placeholders[i] = "?"
		}
		placeholderStr = strings.Join(placeholders, ",")

		query = fmt.Sprintf("%s WHERE %s (%s)", query, by, placeholderStr)
	}

	if limit > 0 {
		limitStr := fmt.Sprintf("LIMIT %d", limit)
		query = fmt.Sprintf("%s %s", query, limitStr)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("error querying db: %v", err)
		return nil, err
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var resources []any
	for rows.Next() {
		var r ut.Volume
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		resources = append(resources, r)
	}

	return resources, nil
}

func getVolume(db *sql.DB, sel, table, by, byvalue string) (any, error) {
	if sel == "" {
		sel = "*"
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", sel, table, by)

	var r ut.Volume
	err := db.QueryRow(query, byvalue).Scan(r.PtrFields()...)
	if err != nil {
		log.Printf("[FSL_DB_getVolume] error scanning row: %v", err)
		return r, err
	}

	return r, nil
}

func getUserVolumes(db *sql.DB, sel, table, by, byvalue string, limit int) ([]any, error) {
	if sel == "" {
		sel = "*"
	}
	query := fmt.Sprintf("SELECT %s FROM %s", sel, table)

	var placeholderStr string
	var args []any
	for _, arg := range strings.Split(strings.TrimSpace(byvalue), ",") {
		args = append(args, arg)
	}
	if by != "" && byvalue != "" {
		byValuesSplit := strings.Split(strings.TrimSpace(byvalue), ",")
		placeholders := make([]string, len(byValuesSplit))
		for i := range byValuesSplit {
			placeholders[i] = "?"
		}
		placeholderStr = strings.Join(placeholders, ",")

		query = fmt.Sprintf("%s WHERE %s (%s)", query, by, placeholderStr)
	}

	if limit > 0 {
		limitStr := fmt.Sprintf("LIMIT %d", limit)
		query = fmt.Sprintf("%s %s", query, limitStr)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("error querying db: %v", err)
		return nil, err
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var resources []any
	for rows.Next() {
		var r ut.UserVolume
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		resources = append(resources, r)
	}

	return resources, nil
}

func getUserVolume(db *sql.DB, sel, table, by, byvalue string) (any, error) {
	if sel == "" {
		sel = "*"
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", sel, table, by)

	var r ut.UserVolume
	err := db.QueryRow(query, byvalue).Scan(r.PtrFields()...)
	if err != nil {
		log.Printf("[FSL_DB_getUv] error scanning row: %v", err)
		return r, err
	}

	return r, nil
}

func getGroupVolumes(db *sql.DB, sel, table, by, byvalue string, limit int) ([]any, error) {
	if sel == "" {
		sel = "*"
	}
	query := fmt.Sprintf("SELECT %s FROM %s", sel, table)

	var placeholderStr string
	var args []any
	for _, arg := range strings.Split(strings.TrimSpace(byvalue), ",") {
		args = append(args, arg)
	}
	if by != "" && byvalue != "" {
		byValuesSplit := strings.Split(strings.TrimSpace(byvalue), ",")
		placeholders := make([]string, len(byValuesSplit))
		for i := range byValuesSplit {
			placeholders[i] = "?"
		}
		placeholderStr = strings.Join(placeholders, ",")

		query = fmt.Sprintf("%s WHERE %s (%s)", query, by, placeholderStr)
	}

	if limit > 0 {
		limitStr := fmt.Sprintf("LIMIT %d", limit)
		query = fmt.Sprintf("%s %s", query, limitStr)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("error querying db: %v", err)
		return nil, err
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var resources []any
	for rows.Next() {
		var r ut.GroupVolume
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		resources = append(resources, r)
	}

	return resources, nil
}

func getGroupVolume(db *sql.DB, sel, table, by, byvalue string) (any, error) {
	if sel == "" {
		sel = "*"
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", sel, table, by)

	var r ut.GroupVolume
	err := db.QueryRow(query, byvalue).Scan(r.PtrFields()...)
	if err != nil {
		log.Printf("error scanning row: %v", err)
		return r, err
	}

	return r, nil
}
