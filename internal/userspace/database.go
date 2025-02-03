package userspace

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	_ "github.com/marcboeker/go-duckdb"
)

const (
	USERSPACE_DB_PATH = "data/db/"
	initSql           = `
    CREATE TABLE IF NOT EXISTS resources (
      rid INTEGER PRIMARY KEY,
      uid INTEGER,
      vid INTEGER,
      gid INTEGER,
      pid INTEGER,
      size INTEGER,
      links INTEGER,
      perms TEXT,
      name TEXT,
      type TEXT,
      created_at DATETIME,
      updated_at DATETIME,
      accessed_at DATETIME
    );

    CREATE TABLE IF NOT EXISTS volumes (
      vid INTEGER PRIMARY KEY,
      path TEXT,
	  dynamic BOOLEAN,
      capacity INTEGER,
      usage INTEGER
    );

	CREATE TABLE IF NOT EXISTS userVolume(
		vid INTEGER,
		uid INTEGER,
		usage INTEGER,
		quota INTEGER,
		updated_at DATETIME
	);

    CREATE SEQUENCE IF NOT EXISTS seq_resourceid START 1;
    CREATE SEQUENCE IF NOT EXISTS seq_volumeid START 1; 
    
    `
)

type DBHandler struct {
	db     *sql.DB
	DBName string
}

func (m *DBHandler) getConn() (*sql.DB, error) {
	if m.db == nil {
		db, err := sql.Open("duckdb", USERSPACE_DB_PATH+m.DBName)
		if err != nil {
			return nil, err
		}
		m.db = db
	}
	return m.db, nil
}

func (m *DBHandler) Close() {
	if m.db != nil {
		m.db.Close()
	}
}

func (m *DBHandler) Init(database_path, volumes_path, capacity string) {
	log.Printf("Initializing %v database", m.DBName)
	_, err := os.Stat("data")
	if err != nil {
		err = os.Mkdir("data", 0o700)
		if err != nil {
			log.Fatalf("failed to make new directory, destructive: %v", err)
		}
	}

	// init the correct local savings directory.
	_, err = os.Stat(database_path)
	if err != nil {
		err = os.Mkdir(database_path, 0o700)
		if err != nil {
			log.Fatalf("failed to make new directory, destructive: %v", err)
		}
	}

	/* perform the init db scripts */
	db, err := m.getConn()
	if err != nil {
		log.Fatalf("couldn't get db connection, destructive: %v", err)
	}

	// init tables
	_, err = db.Exec(initSql)
	if err != nil {
		log.Fatalf("failed to init db, destrcutive: %v", err)
	}

	var exists bool
	err = db.QueryRow("SELECT COUNT(*) > 0 FROM volumes").Scan(&exists)
	if err != nil {
		log.Fatalf("failed to scan exists query: %v", err)
	}

	if !exists {

		// insert an init volume
		vquery := `
		INSERT INTO 
			volumes (vid, path, dynamic, capacity, usage)
		VALUES
			(nextval('seq_volumeid'), ?, ?, ?, ?)
		 `

		_, err = db.Exec(vquery, volumes_path, "true", capacity, 0)
		if err != nil {
			log.Fatalf("failed to insert init data, destructive: %v", err)
		}
	}
}

func (m *DBHandler) InsertUserVolume(uv UserVolume) error {
	db, err := m.getConn()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `
		INSERT INTO userVolume (vid, uid, usage, quota, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err = db.Exec(query, uv.Vid, uv.Uid, uv.Usage, uv.Quota, uv.Updated_at)
	if err != nil {
		return fmt.Errorf("failed to insert user volume: %v", err)
	}

	return nil
}

func (m *DBHandler) DeleteUserVolumeByUid(uid int) error {
	db, err := m.getConn()
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

func (m *DBHandler) DeleteUserVolumeByVid(vid int) error {
	db, err := m.getConn()
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

func (m *DBHandler) UpdateUserVolume(uv UserVolume) error {
	var setClauses []string
	var params []interface{}

	val := reflect.ValueOf(uv)
	typ := reflect.TypeOf(uv)

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldName := typ.Field(i).Name

		if isEmpty(field) {
			continue
		}

		columnName := toSnakeCase(fieldName)
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", columnName))
		params = append(params, field.Interface())
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no fields to update")
	}

	params = append(params, uv.Vid, uv.Uid)

	query := fmt.Sprintf(`
		UPDATE userVolume
		SET %s
		WHERE vid = ? AND uid = ?
	`, strings.Join(setClauses, ", "))

	db, err := m.getConn()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	_, err = db.Exec(query, uv.Usage, uv.Quota, uv.Updated_at, uv.Vid, uv.Uid)
	if err != nil {
		return fmt.Errorf("failed to update user volume: %v", err)
	}

	return nil
}

func (m *DBHandler) GetUserVolumesByUserIds(uids []int) (interface{}, error) {
	db, err := m.getConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `
			SELECT * FROM userVolume WHERE uid IN (?` + strings.Repeat(",?", len(uids)-1) + `)
		`

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

func (m *DBHandler) GetUserVolumesByVolumeIds(vids []int) (interface{}, error) {
	db, err := m.getConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	query := `
			SELECT * FROM userVolume WHERE vid IN (?` + strings.Repeat(",?", len(vids)-1) + `)
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

func (m *DBHandler) GetVolumes() ([]Volume, error) {
	db, err := m.getConn()
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

func (m *DBHandler) UpdateVolume(volume Volume) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	query := `
		UPDATE 
			volumes
		SET
			path = ?, dynamic = ?, capacity = ?, usage = ?
		WHERE
			vid = ?;
	`

	_, err = db.Exec(query, volume.Path, volume.Dynamic, volume.Capacity, volume.Usage)
	if err != nil {
		log.Printf("error on query execution: %v", err)
		return err
	}

	return nil
}

func (m *DBHandler) DeleteVolume(vid int) error {
	db, err := m.getConn()
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

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}
	return nil
}

func (m *DBHandler) InsertVolume(volume Volume) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	_, err = db.Exec(`
		INSERT INTO 
			volumes (path, dynamic, capacity, usage) 
		VALUES (nextval('seq_volumeid'), ?, ?, ?, ?)`, volume.FieldsNoId()...)
	if err != nil {
		log.Printf("error upon executing insert query: %v", err)
		return err
	}
	return nil
}

func (m *DBHandler) InsertResource(resource Resource) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	query := `
    INSERT INTO 
      resources (rid, uid, vid, gid, pid, size, links, perms, name, type, created_at, updated_at, accessed_at)
    VALUES (nextval('seq_resourceid'), ?, ?, ?, ?, ?, ?, ?, ?, ? ,? ,?, ?);
  `
	_, err = db.Exec(query, resource.FieldsNoId()...)
	if err != nil {
		log.Printf("failed to insert the resource: %v", err)
		return err
	}

	return nil
}

func (m *DBHandler) InsertResourceUniqueName(resource Resource) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	// Check if a resource with the same name and UID already exists
	queryCheck := `SELECT 1 FROM resources WHERE name = ? LIMIT 1;`
	var exists int
	err = db.QueryRow(queryCheck, resource.Name).Scan(&exists)

	if err == nil {
		log.Printf("resource with name '%s' already exists", resource.Name)
		return fmt.Errorf("resource with name '%s' already exists", resource.Name)
	} else if err != sql.ErrNoRows {
		// Return any other query errors
		log.Printf("error checking name uniqueness: %v", err)
		return err
	}

	// Insert the resource if no duplicate was found
	queryInsert := `
    INSERT INTO 
      resources (rid, uid, vid, gid, pid, size, links, perms, name, type, created_at, updated_at, accessed_at)
    VALUES (nextval('seq_resourceid'), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
  `

	_, err = db.Exec(queryInsert, resource.FieldsNoId()...)
	if err != nil {
		log.Printf("failed to insert the resource: %v", err)
		return err
	}

	return nil
}

func (m *DBHandler) InsertResources(resources []Resource) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transacation: %v", err)
		return err
	}

	query := `
    INSERT INTO 
      resources (rid, uid, vid, gid, pid, size, links, perms, name, type, created_at, updated_at, accessed_at)
    VALUES (nextval('seq_resourceid'), ?, ?, ?, ?, ?, ?, ?, ?, ? ,? ,?, ?);`

	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("error preparing transaction: %v", err)
		return err
	}
	defer stmt.Close()

	for _, r := range resources {
		_, err = stmt.Exec(r.FieldsNoId()...)
		if err != nil {
			log.Printf("error executing transaction: %v", err)
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

func (m *DBHandler) InsertResourcesUniqueName(resources []Resource) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)
		return err
	}

	// Prepare the SELECT query to check if the resource exists
	queryCheck := `SELECT 1 FROM resources WHERE name = ? LIMIT 1;`
	stmtCheck, err := tx.Prepare(queryCheck)
	if err != nil {
		log.Printf("error preparing uniqueness check statement: %v", err)
		return err
	}
	defer stmtCheck.Close()

	// Prepare the INSERT statement
	queryInsert := `
    INSERT INTO 
      resources (rid, uid, vid, gid, pid, size, links, perms, name, type, created_at, updated_at, accessed_at)
    VALUES (nextval('seq_resourceid'), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	stmtInsert, err := tx.Prepare(queryInsert)
	if err != nil {
		log.Printf("error preparing insert statement: %v", err)
		return err
	}
	defer stmtInsert.Close()

	for _, r := range resources {
		var exists int

		log.Printf("resource: %+v", r)
		// Check if the resource already exists
		err = stmtCheck.QueryRow(r.Name).Scan(&exists)
		if err == nil {
			log.Printf("resource with name '%s' already exists", r.Name)
			return fmt.Errorf("resource with name '%s' already exists", r.Name)
		} else if err != sql.ErrNoRows {
			// If any other error occurs during the query, return it
			log.Printf("error checking name uniqueness: %v", err)
			return err
		}

		// If the resource doesn't exist, insert it
		_, err = stmtInsert.Exec(r.FieldsNoId()...)
		if err != nil {
			log.Printf("error executing insert: %v", err)
			return err
		}
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func (m *DBHandler) GetAllResourcesAt(path string) ([]Resource, error) {
	db, err := m.getConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return nil, err
	}

	rows, err := db.Query(`
    SELECT 
      * 
    FROM 
      resources 
    WHERE 
      name LIKE ?
  `, path)
	if err != nil {
		log.Printf("error querying db: %v", err)
		return nil, err
	}
	defer rows.Close()

	var resources []Resource
	for rows.Next() {
		var r Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("failed to scan resource: %v", err)
			return nil, err
		}

		resources = append(resources, r)
	}

	return resources, nil
}

func (m *DBHandler) GetAllResources() ([]Resource, error) {
	db, err := m.getConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return nil, err
	}
	rows, err := db.Query(`
    SELECT
      *
    FROM 
      resources`)
	if err != nil {
		log.Printf("error querying db: %v", err)
		return nil, err
	}
	defer rows.Close()

	var resources []Resource
	for rows.Next() {
		var r Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		resources = append(resources, r)
	}

	return resources, nil
}

func (m *DBHandler) GetResourceByIds(rids []int) ([]Resource, error) {
	db, err := m.getConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return nil, err
	}

	rows, err := db.Query(`
    SELECT
      *
    FROM 
      resources 
    WHERE 
      rid IN (?)`, rids)
	if err != nil {
		log.Printf("error querying db: %v", err)
		return nil, err
	}

	var resources []Resource
	for rows.Next() {
		var r Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		resources = append(resources, r)
	}
	return resources, nil
}

func (m *DBHandler) GetResourceByFilepath(filepath string) (*Resource, error) {
	db, err := m.getConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return nil, err
	}

	var resource Resource
	err = db.QueryRow("SELECT * FROM resources WHERE name = ?", filepath).Scan(resource.PtrFields()...)
	if err != nil {
		log.Printf("error scanning resource: %v", err)
		return nil, err
	}

	return &resource, nil
}

func (m *DBHandler) DeleteResourcesByIds(rids []string) error {
	// can't have empty arg (might be destructive)
	if rids == nil || len(rids) == 0 {
		log.Printf("empty argument, returning...")
		return fmt.Errorf("must provide input ids")
	}

	db, err := m.getConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("error starting transaction: %v", err)
		return err
	}

	placeholders := make([]string, len(rids))
	args := make([]interface{}, len(rids))
	for i := range rids {
		placeholders[i] = "?"
		args[i] = rids[i]
	}

	query := fmt.Sprintf("DELETE FROM resources WHERE rid IN (%s)", strings.Join(placeholders, ","))

	res, err := tx.Exec(query, args...)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to get rows affected: %v", err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}
	log.Printf("deleted %v rows", rAff)

	return nil
}

func (m *DBHandler) DeleteResourceByName(name string) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("error starting transaction: %v", err)
		return err
	}

	res, err := tx.Exec("DELETE FROM resources WHERE name = ?", name)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to get rows affected")
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}
	log.Printf("deleted %v rows", rAff)

	return nil
}

func (m *DBHandler) UpdateResourceById(rid int, r Resource) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("error starting transaction: %v", err)
		return err
	}

	// we should check which fields are empty and not update those...

	var setClauses []string
	var params []interface{}

	val := reflect.ValueOf(r)
	typ := reflect.TypeOf(r)

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldName := typ.Field(i).Name

		if isEmpty(field) {
			continue
		}

		columnName := toSnakeCase(fieldName)
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", columnName))
		params = append(params, field.Interface())
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no fields to update")
	}

	params = append(params, rid)

	query := fmt.Sprintf(`
    UPDATE 
      resources 
    SET 
      %s 
    WHERE 
      rid = ?
  `, strings.Join(setClauses, ", "))

	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("error preparing transaction: %v", err)
		return err
	}

	stmt.Exec()

	return nil
}

func (m *DBHandler) InsertVolumes(volumes []Volume) error {
	db, err := m.getConn()
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

func (m *DBHandler) SelectAllVolumes() ([]Volume, error) {
	db, err := m.getConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return nil, err
	}

	query := `
    SELECT *
    FROM volumes
  `
	rows, err := db.Query(query, nil)
	if err != nil {
		log.Printf("error querying: %v", err)
		return nil, err
	}

	var volumes []Volume
	for rows.Next() {
		var volume Volume
		err := rows.Scan(volume.PtrFields())
		if err != nil {
			log.Printf("failed to scan volume: %v", err)
			return nil, err
		}
		volumes = append(volumes, volume)
	}

	return volumes, nil
}

func (m *DBHandler) DeleteVolumeByIds(ids []int) error {
	if ids == nil {
		return fmt.Errorf("must provide ids")
	}

	db, err := m.getConn()
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

// Helper function to determine if a value is empty
func isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	default:
		return v.IsZero() // General case for other types
	}
}
