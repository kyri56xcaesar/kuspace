package userspace

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

/* an Interface regarding the "Resource" type handling in the Database */
type ResourcesDBHandler interface {
	InsertResource(resource Resource) error
	InsertResources(resources []Resource) error
	InsertResourceUniqueName(resource Resource) error
	InsertResourcesUniqueName(resources []Resource) error
	GetAllResources() ([]Resource, error)
	GetResourcesByIds(rids []int) ([]Resource, error)
	GetResourceByFilepath(filepath string) (Resource, error)
	GetAllResourcesAt(path string) ([]Resource, error)
	DeleteResourcesByIds(rids []string) (int64, error)
	DeleteResourceByName(name string) error
	UpdateResourceNameById(rid, name string) error
	UpdateResourcePermsById(rid, perms string) error
	UpdateResourceOwnerById(rid, uid int) error
	UpdateResourceGroupById(rid, gid int) error
}

/*
	 a reference struct implementing all the methods and also holding a database connection ref.
		for duckdb
*/
type Rh struct {
	dbh *DBHandler
}

/* database call handlers regarding the Resource table */
func (m *Rh) InsertResource(resource Resource) error {
	db, err := m.dbh.getConn()
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

func (m *Rh) InsertResourceUniqueName(resource Resource) error {
	db, err := m.dbh.getConn()
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

func (m *Rh) InsertResources(resources []Resource) error {
	db, err := m.dbh.getConn()
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

func (m *Rh) InsertResourcesUniqueName(resources []Resource) error {
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

func (m *Rh) GetAllResourcesAt(path string) ([]Resource, error) {
	db, err := m.dbh.getConn()
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

func (m *Rh) GetAllResources() ([]Resource, error) {
	db, err := m.dbh.getConn()
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

func (m *Rh) GetResourcesByIds(rids []int) ([]Resource, error) {
	db, err := m.dbh.getConn()
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

func (m *Rh) GetResourceByFilepath(filepath string) (Resource, error) {
	var resource Resource

	db, err := m.dbh.getConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return resource, err
	}

	err = db.QueryRow("SELECT * FROM resources WHERE name = ?", filepath).Scan(resource.PtrFields()...)
	if err != nil {
		log.Printf("error scanning resource: %v", err)
		return resource, err
	}

	return resource, nil
}

func (m *Rh) DeleteResourcesByIds(rids []string) (int64, error) {
	// can't have empty arg (might be destructive)
	if len(rids) == 0 {
		log.Printf("empty argument, returning...")
		return 0, fmt.Errorf("must provide input ids")
	}

	db, err := m.dbh.getConn()
	if err != nil {
		log.Printf("error getting db connection: %v", err)
		return 0, err
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("error starting transaction: %v", err)
		return 0, err
	}

	placeholders := make([]string, len(rids))
	args := make([]interface{}, len(rids))
	for i := range rids {
		placeholders[i] = "?"
		args[i] = rids[i]
	}

	// we want to save the size of this resource before we delete it.
	var size int64

	squery := fmt.Sprintf("SELECT size FROM resources WHERE rid IN (%s)", strings.Join(placeholders, ","))
	query := fmt.Sprintf("DELETE FROM resources WHERE rid IN (%s)", strings.Join(placeholders, ","))

	sres, err := tx.Query(squery, args...)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		return 0, err
	}

	res, err := tx.Exec(query, args...)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		return 0, err
	}

	for sres.Next() {
		var current_size int64
		err = sres.Scan(&current_size)
		if err != nil {
			log.Printf("error scanning size of resource: %v", err)
			return 0, err
		}
		size += current_size
	}
	defer sres.Close()

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to get rows affected: %v", err)
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return 0, err
	}
	log.Printf("deleted %v rows", rAff)

	return size, nil
}

func (m *Rh) DeleteResourceByName(name string) error {
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

func (m *Rh) UpdateResourceNameById(rid, name string) error {
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

	query := `
    UPDATE 
      resources 
    SET 
      name = ?
    WHERE 
      rid = ?;
  `

	res, err := tx.Exec(query, name, rid)
	if err != nil {
		log.Printf("error executing query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to get rows affected")
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("error committing transaction: %v", err)
		return err
	}
	log.Printf("updated %v rows", rAff)

	return nil
}

func (m *Rh) UpdateResourcePermsById(rid, perms string) error {
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

	query := `
    UPDATE 
      resources 
    SET 
      perms = ?
    WHERE 
      rid = ?;
  `

	res, err := tx.Exec(query, perms, rid)
	if err != nil {
		log.Printf("error executing query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to get rows affected")
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("error committing transaction: %v", err)
		return err
	}
	log.Printf("updated %v rows", rAff)

	return nil
}

func (m *Rh) UpdateResourceOwnerById(rid, uid int) error {
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

	query := `
    UPDATE 
      resources 
    SET 
      uid = ?
    WHERE 
      rid = ?;
  `

	res, err := tx.Exec(query, uid, rid)
	if err != nil {
		log.Printf("error executing query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to get rows affected")
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("error committing transaction: %v", err)
		return err
	}
	log.Printf("updated %v rows", rAff)

	return nil
}

func (m *Rh) UpdateResourceGroupById(rid, gid int) error {
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

	query := `
    UPDATE 
      resources 
    SET 
      gid = ?
    WHERE 
      rid = ?;
  `

	res, err := tx.Exec(query, gid, rid)
	if err != nil {
		log.Printf("error executing query: %v", err)
		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to get rows affected")
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("error committing transaction: %v", err)
		return err
	}
	log.Printf("updated %v rows", rAff)

	return nil
}
