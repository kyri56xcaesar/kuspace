package userspace

import (
	"fmt"
	"log"
	"strings"
	"time"
)

/* this is what a Volume Database handler should implement*/
type VolumesDBHandler interface {
	GetVolumes() ([]Volume, error)
	GetVolumeByVid(vid int) (Volume, error)
	UpdateVolume(volume Volume) error
	DeleteVolume(vid int) error
	InsertVolume(volume Volume) error
	InsertVolumes(volumes []Volume) error
	DeleteVolumeByIds(ids []int) error
}

/*
a reference struct implementing all the handlers for the volume data type in the db
*/
type Vh struct {
	dbh *DBHandler
}

/* database call handlers regarding the Volume table */
func (m *Vh) GetVolumes() ([]Volume, error) {
	db, err := m.dbh.getConn()
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

	var volumes []Volume
	for rows.Next() {
		var v Volume
		err = rows.Scan(v.PtrFields()...)
		if err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		volumes = append(volumes, v)
	}

	return volumes, nil
}

func (m *Vh) GetVolumeByVid(vid int) (Volume, error) {
	db, err := m.dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return Volume{}, err
	}
	var volume Volume
	err = db.QueryRow(`SELECT * FROM volumes WHERE vid = ?`, vid).Scan(volume.PtrFields()...)
	if err != nil {
		log.Printf("failed to scan result query: %v", err)
		return Volume{}, err
	}
	return volume, nil
}

func (m *Vh) UpdateVolume(volume Volume) error {
	db, err := m.dbh.getConn()
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

func (m *Vh) DeleteVolume(vid int) error {
	db, err := m.dbh.getConn()
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

func (m *Vh) InsertVolume(volume Volume) error {
	db, err := m.dbh.getConn()
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

func (m *Vh) InsertVolumes(volumes []Volume) error {
	db, err := m.dbh.getConn()
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

func (m *Vh) DeleteVolumeByIds(ids []int) error {
	if ids == nil {
		return fmt.Errorf("must provide ids")
	}

	db, err := m.dbh.getConn()
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

/* User Volume and Group Volume Database related */
type UserVolumeDBHandler interface {
	InsertUserVolume(uv UserVolume) error
	InsertUserVolumes(uvs []UserVolume) error
	DeleteUserVolumeByUid(uid int) error
	DeleteUserVolumeByVid(vid int) error
	UpdateUserVolume(uv UserVolume) error
	UpdateUserVolumeQuotaByUid(quota float32, uid int) error
	UpdateUserVolumeUsageByUid(usage float32, uid int) error
	UpdateUserVolumeQuotaAndUsageByUid(usage, quota float32, uid int) error
	GetUserVolumes() (interface{}, error)
	GetUserVolumeByUid(uid int) (UserVolume, error)
	GetUserVolumesByUserIds(uids []string) (interface{}, error)
	GetUserVolumesByVolumeIds(vids []string) (interface{}, error)
	GetUserVolumesByUidsAndVids(uids, vids []string) (interface{}, error)
}

type GroupVolumeDBHandler interface {
	InsertGroupVolume(gv GroupVolume) error
	InsertGroupVolumes(gvs []GroupVolume) error
	DeleteGroupVolumeByGid(gid int) error
	DeleteGroupVolumeByVid(vid int) error
	UpdateGroupVolume(gv GroupVolume) error
	UpdateGroupVolumes(gvs []GroupVolume) error
	UpdateGroupVolumesUsageByGids(gids []string) error
	GetGroupVolumes() (interface{}, error)
	GetGroupVolumeByGid(gid int) (GroupVolume, error)
	GetGroupVolumesByGroupIds(gids []string) (interface{}, error)
	GetGroupVolumesByVolumeIds(vids []string) (interface{}, error)
	GetGroupVolumesByVidsAndGids(gids, vids []string) (interface{}, error)
}

type UVh struct {
	dbh *DBHandler
}

type GVh struct {
	dbh *DBHandler
}

/* database call handlers regarding the UserVolume table */
/* UNIQUE (vid, uid) pair*/
func (m *UVh) InsertUserVolume(uv UserVolume) error {
	db, err := m.dbh.getConn()
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

func (m *UVh) InsertUserVolumes(uvs []UserVolume) error {
	db, err := m.dbh.getConn()
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

func (m *UVh) DeleteUserVolumeByUid(uid int) error {
	db, err := m.dbh.getConn()
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

func (m *UVh) DeleteUserVolumeByVid(vid int) error {
	db, err := m.dbh.getConn()
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

func (m *UVh) UpdateUserVolume(uv UserVolume) error {

	db, err := m.dbh.getConn()
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

func (m *UVh) UpdateUserVolumeQuotaByUid(quota float32, uid int) error {
	db, err := m.dbh.getConn()
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

func (m *UVh) UpdateUserVolumeUsageByUid(usage float32, uid int) error {
	db, err := m.dbh.getConn()
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

func (m *UVh) UpdateUserVolumeQuotaAndUsageByUid(usage, quota float32, uid int) error {
	db, err := m.dbh.getConn()
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

func (m *UVh) GetUserVolumes() (interface{}, error) {
	db, err := m.dbh.getConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `SELECT * FROM userVolume`

	rows, err := db.Query(query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query user volumes: %v", err)
	}
	defer rows.Close()

	var userVolumes []UserVolume
	for rows.Next() {
		var uv UserVolume
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

func (m *UVh) GetUserVolumeByUid(uid int) (UserVolume, error) {
	db, err := m.dbh.getConn()
	if err != nil {
		return UserVolume{}, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `SELECT * FROM userVolume WHERE uid = ?`

	var userVolume UserVolume
	err = db.QueryRow(query, uid).Scan(userVolume.PtrFields()...)
	if err != nil {
		return UserVolume{}, fmt.Errorf("failed to query user volume: %v", err)
	}
	return userVolume, nil
}

func (m *UVh) GetUserVolumesByUserIds(uids []string) (interface{}, error) {
	db, err := m.dbh.getConn()
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

	var userVolumes []UserVolume
	for rows.Next() {
		var uv UserVolume
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

func (m *UVh) GetUserVolumesByVolumeIds(vids []string) (interface{}, error) {
	db, err := m.dbh.getConn()
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

	var userVolumes []UserVolume
	for rows.Next() {
		var uv UserVolume
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

func (m *UVh) GetUserVolumesByUidsAndVids(uids, vids []string) (interface{}, error) {
	db, err := m.dbh.getConn()
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

	var userVolumes []UserVolume
	for rows.Next() {
		var uv UserVolume
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
func (m *GVh) InsertGroupVolume(gv GroupVolume) error {
	db, err := m.dbh.getConn()
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

func (m *GVh) InsertGroupVolumes(gvs []GroupVolume) error {
	db, err := m.dbh.getConn()
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

func (m *GVh) DeleteGroupVolumeByGid(gid int) error {
	db, err := m.dbh.getConn()
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

func (m *GVh) DeleteGroupVolumeByVid(vid int) error {
	db, err := m.dbh.getConn()
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

func (m *GVh) UpdateGroupVolume(gv GroupVolume) error {

	db, err := m.dbh.getConn()
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

func (m *GVh) UpdateGroupVolumes(gvs []GroupVolume) error {
	if len(gvs) == 0 {
		return nil // No updates needed
	}

	log.Printf("incoming gvs: %+v", gvs)

	query := `
		UPDATE groupVolume
		SET usage = ?, quota = ?, updated_at = ?
		WHERE vid = ? AND gid = ?
	`

	db, err := m.dbh.getConn()
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

func (m *GVh) UpdateGroupVolumesUsageByGids(gids []string) error {
	query := `
    UPDATE groupVolume 
    SET usage = ? 
    WHERE gid IN (?
    ` + strings.Repeat(", ?", len(gids)-1) + `)`

	db, err := m.dbh.getConn()
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

func (m *GVh) UpdateGroupVolumeQuotaByGid(quota float32, gid int) error {
	db, err := m.dbh.getConn()
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

func (m *GVh) UpdateGroupVolumeUsageByGid(usage float32, gid int) error {
	db, err := m.dbh.getConn()
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

func (m *GVh) UpdateGroupVolumeQuotaAndUsageByUid(usage, quota float32, gid int) error {
	db, err := m.dbh.getConn()
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

func (m *GVh) GetGroupVolumes() (interface{}, error) {
	db, err := m.dbh.getConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `SELECT * FROM groupVolume`

	rows, err := db.Query(query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query group volumes: %v", err)
	}
	defer rows.Close()

	var groupVolumes []GroupVolume
	for rows.Next() {
		var gv GroupVolume
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

func (m *GVh) GetGroupVolumeByGid(gid int) (GroupVolume, error) {
	db, err := m.dbh.getConn()
	if err != nil {
		return GroupVolume{}, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `SELECT * FROM groupVolume WHERE gid = ?`
	var groupVolume GroupVolume
	err = db.QueryRow(query, gid).Scan(groupVolume.PtrFields()...)
	if err != nil {
		return GroupVolume{}, fmt.Errorf("failed to query group volumes: %v", err)
	}

	return groupVolume, nil
}

func (m *GVh) GetGroupVolumesByGroupIds(gids []string) (interface{}, error) {
	db, err := m.dbh.getConn()
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

	var groupVolumes []GroupVolume
	for rows.Next() {
		var gv GroupVolume
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

func (m *GVh) GetGroupVolumesByVolumeIds(vids []string) (interface{}, error) {
	db, err := m.dbh.getConn()
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

	var groupVolumes []GroupVolume
	for rows.Next() {
		var gv GroupVolume
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

func (m *GVh) GetGroupVolumesByVidsAndGids(vids, gids []string) (interface{}, error) {
	db, err := m.dbh.getConn()
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

	var groupVolumes []GroupVolume
	for rows.Next() {
		var gv GroupVolume
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
