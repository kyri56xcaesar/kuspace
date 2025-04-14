package fslite

/*
	database call handlers for "volumes"
	"userspace.db"

	@used by the api
*/

import (
	"fmt"
	"log"
	"strings"
	"time"

	ut "kyri56xcaesar/myThesis/internal/utils"
)

/* database call handlers regarding the Volume table */
func (fsl *FsLite) GetVolumes() ([]ut.Volume, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return nil, err
	}
	rows, err := db.Query(`
    SELECT
      *
    FROM 
      volumes`)
	if err != nil {
		log.Printf("error querying db: %v", err)
		return nil, err
	}
	defer rows.Close()

	var volumes []ut.Volume
	for rows.Next() {
		var v ut.Volume
		err = rows.Scan(v.PtrFields()...)
		if err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		volumes = append(volumes, v)
	}

	return volumes, nil
}

func (fsl *FsLite) GetVolumeByVid(vid int) (ut.Volume, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return ut.Volume{}, err
	}
	var volume ut.Volume
	err = db.QueryRow(`SELECT * FROM volumes WHERE vid = ?`, vid).Scan(volume.PtrFields()...)
	if err != nil {
		log.Printf("failed to scan result query: %v", err)
		return ut.Volume{}, err
	}
	return volume, nil
}

func (fsl *FsLite) UpdateVolume(volume ut.Volume) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	query := `
		UPDATE 
			volumes
		SET
			name = ?, path = ?, dynamic = ?, capacity = ?, usage = ?
		WHERE
			vid = ?;
	`

	_, err = db.Exec(query, volume.Name, volume.Path, volume.Dynamic, volume.Capacity, volume.Usage, volume.Vid)
	if err != nil {
		log.Printf("error on query execution: %v", err)
		return err
	}

	return nil
}

func (fsl *FsLite) DeleteVolume(vid int) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)
		return err
	}
	_, err = tx.Exec("DELETE FROM volumes WHERE vid = ?", vid)
	if err != nil {
		log.Printf("failed to execute delete query: %v", err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}
	return nil
}

func (fsl *FsLite) InsertVolume(volume ut.Volume) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	_, err = db.Exec(`
		INSERT INTO 
			volumes (path, dynamic, capacity, usage) 
		VALUES (nextval('seq_volumeid'), ?, ?, ?, ?, ?)`, volume.FieldsNoId()...)
	if err != nil {
		log.Printf("error upon executing insert query: %v", err)
		return err
	}
	return nil
}

func (fsl *FsLite) InsertVolumes(volumes []ut.Volume) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("error starting transaction: %v", err)
		return err
	}

	placeholder := strings.Repeat("(nextval('seq_volumeid'), ?, ?, ?),", len(volumes))
	query := fmt.Sprintf(`
    INSERT INTO 
      volumes (vid, path, capacity, usage)
    VALUES %s`, placeholder[:len(placeholder)-1])

	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("error preparing transaction: %v", err)
		return err
	}
	defer stmt.Close()

	for _, v := range volumes {
		_, err = stmt.Exec(v.Path, v.Capacity, v.Usage)
		if err != nil {
			log.Printf("error executing transaction: %v", err)
			tx.Rollback()
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func (fsl *FsLite) DeleteVolumeByIds(ids []int) error {
	if ids == nil {
		return fmt.Errorf("must provide ids")
	}

	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("error getting database connetion")
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("error starting transaction: %v", err)
		return err
	}

	res, err := tx.Exec("DELETE FROM volumes WHERE vid IN (?)", ids)
	if err != nil {
		log.Printf("failed to exec deleteion query: %v", err)
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to retrieve rows affected: %v", err)
		return err
	}

	log.Printf("deleted %v entries", rowsAffected)

	return nil
}

/* database call handlers regarding the UserVolume table */
/* UNIQUE (vid, uid) pair*/
func (fsl *FsLite) InsertUserVolume(uv ut.UserVolume) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}
	// check for uniquness
	var exists bool
	err = db.QueryRow(`SELECT 1 FROM userVolume WHERE vid = ? AND uid = ? LIMIT 1;`, uv.Vid, uv.Uid).Scan(&exists)
	if exists {
		if err == nil {
			return fmt.Errorf("already exists")
		}
		log.Printf("error checking for uniqunes or not unique: %v", err)
		return fmt.Errorf("error checking for uniqueness or not unique pair: %v", err)
	}

	query := `
		INSERT INTO userVolume (vid, uid, usage, quota, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")

	_, err = db.Exec(query, uv.Vid, uv.Uid, uv.Usage, uv.Quota, currentTime)
	if err != nil {
		return fmt.Errorf("failed to insert user volume: %v", err)
	}

	return nil
}

func (fsl *FsLite) InsertUserVolumes(uvs []ut.UserVolume) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return err
	}
	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")

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
	defer stmt.Close()

	for _, uv := range uvs {
		uv.Updated_at = currentTime
		_, err = stmt.Exec(uv.Fields()...)
		if err != nil {
			log.Printf("error executing transaction: %v", err)
			tx.Rollback()
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func (fsl *FsLite) DeleteUserVolumeByUid(uid int) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `DELETE FROM userVolume WHERE uid = ?`

	_, err = db.Exec(query, uid)
	if err != nil {
		return fmt.Errorf("failed to delete user volume: %v", err)
	}

	return nil
}

func (fsl *FsLite) DeleteUserVolumeByVid(vid int) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `DELETE FROM userVolume WHERE vid = ?`

	_, err = db.Exec(query, vid)
	if err != nil {
		return fmt.Errorf("failed to delete user volume: %v", err)
	}

	return nil
}

func (fsl *FsLite) UpdateUserVolume(uv ut.UserVolume) error {

	db, err := fsl.dbh.GetConn()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `
		UPDATE userVolume
		SET usage = ?, quota = ?, updated_at = ?
		WHERE vid = ? AND uid = ?
	`

	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")

	_, err = db.Exec(query, uv.Usage, uv.Quota, currentTime, uv.Vid, uv.Uid)
	if err != nil {
		return fmt.Errorf("failed to update user volume: %v", err)
	}

	return nil
}

func (fsl *FsLite) UpdateUserVolumeQuotaByUid(quota float32, uid int) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve database connection: %v", err)
		return err
	}

	query := `
    UPDATE 
      userVolume
    SET
      quota = ? 
    WHERE 
      uid = ?
      
  `

	res, err := db.Exec(query, quota, uid)
	if err != nil {
		log.Printf("failed to exec query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to retrieve info about rows affected")
		return err
	}
	log.Printf("rows affected: %v", rAff)
	return nil
}

func (fsl *FsLite) UpdateUserVolumeUsageByUid(usage float32, uid int) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve database connection: %v", err)
		return err
	}

	query := `
    UPDATE 
      userVolume
    SET
      usage = ? 
    WHERE 
      uid = ?
      
  `

	res, err := db.Exec(query, usage, uid)
	if err != nil {
		log.Printf("failed to exec query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to retrieve info about rows affected")
		return err
	}
	log.Printf("rows affected: %v", rAff)
	return nil
}

func (fsl *FsLite) UpdateUserVolumeQuotaAndUsageByUid(usage, quota float32, uid int) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve database connection: %v", err)
		return err
	}

	query := `
    UPDATE 
      userVolume
    SET
      usage = ?, quota = ? 
    WHERE 
      uid = ?
      
  `

	res, err := db.Exec(query, usage, quota, uid)
	if err != nil {
		log.Printf("failed to exec query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to retrieve info about rows affected")
		return err
	}
	log.Printf("rows affected: %v", rAff)
	return nil
}

func (fsl *FsLite) GetUserVolumes() (interface{}, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `SELECT * FROM userVolume`

	rows, err := db.Query(query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %v", err)
	}
	defer rows.Close()

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

func (fsl *FsLite) GetUserVolumeByUid(uid int) (ut.UserVolume, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return ut.UserVolume{}, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `SELECT * FROM userVolume WHERE uid = ?`

	var userVolume ut.UserVolume
	err = db.QueryRow(query, uid).Scan(userVolume.PtrFields()...)
	if err != nil {
		return ut.UserVolume{}, fmt.Errorf("failed to query user volume: %v", err)
	}
	return userVolume, nil
}

func (fsl *FsLite) GetUserVolumesByUserIds(uids []string) (interface{}, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `
			SELECT * FROM userVolume WHERE uid IN (?` + strings.Repeat(",?", len(uids)-1) + `)
		`

	if len(uids) == 1 && uids[0] == "*" {
		query = `SELECT * FROM userVolume;`
	}

	args := make([]interface{}, len(uids))
	for i, uid := range uids {
		args[i] = uid
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %v", err)
	}
	defer rows.Close()

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

func (fsl *FsLite) GetUserVolumesByVolumeIds(vids []string) (interface{}, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `
			SELECT * FROM userVolume WHERE vid IN (?` + strings.Repeat(",?", len(vids)-1) + `)
		`

	if len(vids) == 1 && vids[0] == "*" {
		query = `SELECT * FROM userVolume;`
	}

	args := make([]interface{}, len(vids))
	for i, uid := range vids {
		args[i] = uid
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %v", err)
	}
	defer rows.Close()

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

func (fsl *FsLite) GetUserVolumesByUidsAndVids(uids, vids []string) (interface{}, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `
		SELECT 
      * 
    FROM 
      userVolume 
    WHERE 
      vid IN (?` + strings.Repeat(",?", len(vids)-1) + `)
    AND 
      uid IN (?` + strings.Repeat(",?", len(uids)-1) + `)
	  	
  `

	args := make([]interface{}, len(vids))
	for i, uid := range vids {
		args[i] = uid
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %v", err)
	}
	defer rows.Close()

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
func (fsl *FsLite) InsertGroupVolume(gv ut.GroupVolume) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	// check for uniquness
	var exists bool
	err = db.QueryRow(`SELECT 1 FROM groupVolume WHERE vid = ? AND gid = ? LIMIT 1;`, gv.Vid, gv.Gid).Scan(&exists)
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

	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")

	_, err = db.Exec(query, gv.Vid, gv.Gid, gv.Usage, gv.Quota, currentTime)
	if err != nil {
		return fmt.Errorf("failed to insert group volume: %v", err)
	}

	return nil
}

func (fsl *FsLite) InsertGroupVolumes(gvs []ut.GroupVolume) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return err
	}
	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")

	tx, err := db.Begin()
	if err != nil {
		log.Printf("error starting transaction: %v", err)
		return err
	}

	placeholder := strings.Repeat("(?, ?, ?, ?, ?),", len(gvs))
	query := fmt.Sprintf(`
    INSERT INTO 
      groupVolume (vid, gid, usage, quota, updated_at)
    VALUES %s`, placeholder[:len(placeholder)-1])

	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("error preparing transaction: %v", err)
		return err
	}
	defer stmt.Close()

	for _, gv := range gvs {
		gv.Updated_at = currentTime
		_, err = stmt.Exec(gv.Fields()...)
		if err != nil {
			log.Printf("error executing transaction: %v", err)
			tx.Rollback()
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func (fsl *FsLite) DeleteGroupVolumeByGid(gid int) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `DELETE FROM groupVolume WHERE gid = ?`

	_, err = db.Exec(query, gid)
	if err != nil {
		return fmt.Errorf("failed to delete group volume: %v", err)
	}

	return nil
}

func (fsl *FsLite) DeleteGroupVolumeByVid(vid int) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `DELETE FROM groupVolume WHERE vid = ?`

	_, err = db.Exec(query, vid)
	if err != nil {
		return fmt.Errorf("failed to delete group volume: %v", err)
	}

	return nil
}

func (fsl *FsLite) UpdateGroupVolume(gv ut.GroupVolume) error {

	db, err := fsl.dbh.GetConn()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}
	query := `
		UPDATE groupVolume
		SET usage = ?, quota = ?, updated_at = ?
		WHERE vid = ? AND gid = ?
	`

	current_time := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")

	_, err = db.Exec(query, gv.Usage, gv.Quota, current_time, gv.Vid, gv.Gid)
	if err != nil {
		return fmt.Errorf("failed to update group volume: %v", err)
	}

	return nil
}

func (fsl *FsLite) UpdateGroupVolumes(gvs []ut.GroupVolume) error {
	if len(gvs) == 0 {
		return nil // No updates needed
	}

	log.Printf("incoming gvs: %+v", gvs)

	query := `
		UPDATE groupVolume
		SET usage = ?, quota = ?, updated_at = ?
		WHERE vid = ? AND gid = ?
	`

	db, err := fsl.dbh.GetConn()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	// Begin a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	// Get current UTC timestamp
	current_time := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")

	// Execute updates within the transaction
	for _, gv := range gvs {
		_, err := stmt.Exec(gv.Usage, gv.Quota, current_time, gv.Vid, gv.Gid)
		if err != nil {
			tx.Rollback() // Rollback on error
			return fmt.Errorf("failed to update group volume (vid=%d, gid=%d): %v", gv.Vid, gv.Gid, err)
		}
	}

	// Commit transaction if all updates succeed
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func (fsl *FsLite) UpdateGroupVolumesUsageByGids(gids []string) error {
	query := `
    UPDATE groupVolume 
    SET usage = ? 
    WHERE gid IN (?
    ` + strings.Repeat(", ?", len(gids)-1) + `)`

	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve the database connection: %v", err)
		return err
	}
	args := make([]interface{}, len(gids))
	for i, v := range gids {
		args[i] = v
	}
	res, err := db.Exec(query, args...)
	if err != nil {
		log.Printf("failed to exec query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to retrieve rows affected: %v", err)
		return err
	}

	log.Printf("len(gids): %v, rAff: %v", len(gids), rAff)
	return nil
}

func (fsl *FsLite) UpdateGroupVolumeQuotaByGid(quota float32, gid int) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve database connection: %v", err)
		return err
	}

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
		log.Printf("failed to exec query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to retrieve info about rows affected")
		return err
	}
	log.Printf("rows affected: %v", rAff)
	return nil
}

func (fsl *FsLite) UpdateGroupVolumeUsageByGid(usage float32, gid int) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve database connection: %v", err)
		return err
	}

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
		log.Printf("failed to exec query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to retrieve info about rows affected")
		return err
	}
	log.Printf("rows affected: %v", rAff)
	return nil
}

func (fsl *FsLite) UpdateGroupVolumeQuotaAndUsageByUid(usage, quota float32, gid int) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve database connection: %v", err)
		return err
	}

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
		log.Printf("failed to exec query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to retrieve info about rows affected")
		return err
	}
	log.Printf("rows affected: %v", rAff)
	return nil
}

func (fsl *FsLite) GetGroupVolumes() (interface{}, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `SELECT * FROM groupVolume`

	rows, err := db.Query(query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %v", err)
	}
	defer rows.Close()

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

func (fsl *FsLite) GetGroupVolumeByGid(gid int) (ut.GroupVolume, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return ut.GroupVolume{}, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `SELECT * FROM groupVolume WHERE gid = ?`
	var groupVolume ut.GroupVolume
	err = db.QueryRow(query, gid).Scan(groupVolume.PtrFields()...)
	if err != nil {
		return ut.GroupVolume{}, fmt.Errorf("failed to query group volumes: %v", err)
	}

	return groupVolume, nil
}

func (fsl *FsLite) GetGroupVolumesByGroupIds(gids []string) (any, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `
			SELECT * FROM groupVolume WHERE gid IN (?` + strings.Repeat(",?", len(gids)-1) + `)
		`
	if len(gids) == 1 && gids[0] == "*" {
		query = `SELECT * FROM groupVolume;`
	}

	args := make([]interface{}, len(gids))
	for i, uid := range gids {
		args[i] = uid
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %v", err)
	}
	defer rows.Close()

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

func (fsl *FsLite) GetGroupVolumesByVolumeIds(vids []string) (interface{}, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `
			SELECT * FROM groupVolume WHERE vid IN (?` + strings.Repeat(",?", len(vids)-1) + `)
		`

	if len(vids) == 1 && vids[0] == "*" {
		query = `SELECT * FROM groupVolume;`
	}
	args := make([]interface{}, len(vids))
	for i, uid := range vids {
		args[i] = uid
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %v", err)
	}
	defer rows.Close()

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

func (fsl *FsLite) GetGroupVolumesByVidsAndGids(vids, gids []string) (any, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `
		SELECT 
      * 
    FROM 
      groupVolume 
    WHERE 
      vid IN (?` + strings.Repeat(",?", len(vids)-1) + `)
    AND 
      gid IN (?` + strings.Repeat(",?", len(gids)-1) + `)
	`

	args := make([]interface{}, len(vids))
	for i, uid := range vids {
		args[i] = uid
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %v", err)
	}
	defer rows.Close()

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

func (fsl *FsLite) InitResourceVolumeSpecific(database_path, volumes_path, capacity string) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Fatalf("couldn't get db connection, destructive: %v", err)
	}
	// specific initialization
	var (
		exists bool
		vid    int64
	)
	err = db.QueryRow("SELECT COUNT(*) > 0 FROM volumes").Scan(&exists)
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
		err := db.QueryRow(vquery, default_volume_name, volumes_path, "true", capacity, 0).Scan(&vid)
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
