package fslite

/*
	database call handlers for "resources"
	"uspace.db"

	@used by the api
*/

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

type ScanFunc[T any] func(*sql.Rows) (T, error)

/* database call handlers regarding the Resource table */
func insertResource(db *sql.DB, resource ut.Resource) error {
	query := `
    INSERT INTO 
      resources (rid, uid, gid, vid, vname, size, links, perms, name, path, type, created_at, updated_at, accessed_at)
	VALUES (nextval('seq_resourceid'), ?, ?, ?, ?, ?, ?, ?, ?, ? ,? ,?, ?, ?);  
	`
	resource.Accessed_at = ut.CurrentTime()
	resource.Created_at = ut.CurrentTime()
	resource.Updated_at = ut.CurrentTime()
	_, err := db.Exec(query, resource.FieldsNoId()...)
	if err != nil {
		log.Printf("failed to insert the resource: %v", err)
		return err
	}
	return nil
}

func insertResourceUniqueName(db *sql.DB, resource ut.Resource) error {

	// Check if a resource with the same name and UID already exists
	queryCheck := `SELECT 1 FROM resources WHERE name = ? LIMIT 1;`
	var exists int
	err := db.QueryRow(queryCheck, resource.Name).Scan(&exists)

	if err == nil {
		log.Printf("resource with name '%s' already exists", resource.Name)
		return fmt.Errorf("resource with name '%s' already exists", resource.Name)
	} else if err != sql.ErrNoRows {
		// Return any other query errors
		log.Printf("error checking name uniqueness: %v", err)
		return err
	}

	resource.Accessed_at = ut.CurrentTime()
	resource.Created_at = ut.CurrentTime()
	resource.Updated_at = ut.CurrentTime()
	// Insert the resource if no duplicate was found
	queryInsert := `
    INSERT INTO 
      resources (rid, uid, gid, vid, vname, size, links, perms, name, path, type, created_at, updated_at, accessed_at)
	VALUES (nextval('seq_resourceid'), ?, ?, ?, ?, ?, ?, ?, ?, ? ,? ,?, ?, ?);
  `

	_, err = db.Exec(queryInsert, resource.FieldsNoId()...)
	if err != nil {
		log.Printf("failed to insert the resource: %v", err)
		return err
	}

	return nil
}

func insertResources(db *sql.DB, resources []ut.Resource) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transacation: %v", err)
		return err
	}

	query := `
    INSERT INTO 
      resources (rid, uid, gid, vid, vname, size, links, perms, name, path, type, created_at, updated_at, accessed_at)
	VALUES (nextval('seq_resourceid'), ?, ?, ?, ?, ?, ?, ?, ?, ? ,? ,?, ?, ?);
	`

	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("error preparing transaction: %v", err)
		return err
	}
	defer stmt.Close()

	for _, r := range resources {
		r.Accessed_at = ut.CurrentTime()
		r.Created_at = ut.CurrentTime()
		r.Updated_at = ut.CurrentTime()
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

func insertResourcesUniqueName(db *sql.DB, resources []ut.Resource) error {
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
      resources (rid, uid, gid, vid, vname, size, links, perms, name, path, type, created_at, updated_at, accessed_at)
	VALUES (nextval('seq_resourceid'), ?, ?, ?, ?, ?, ?, ?, ?, ? ,? ,?, ?, ?);
	`
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
		r.Accessed_at = ut.CurrentTime()
		r.Created_at = ut.CurrentTime()
		r.Updated_at = ut.CurrentTime()
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

func getAllResourcesAt(db *sql.DB, path string) ([]ut.Resource, error) {
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

	var resources []ut.Resource
	for rows.Next() {
		var r ut.Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("failed to scan resource: %v", err)
			return nil, err
		}

		resources = append(resources, r)
	}

	return resources, nil
}

func getAllResources(db *sql.DB) ([]ut.Resource, error) {
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

	var resources []ut.Resource
	for rows.Next() {
		var r ut.Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		resources = append(resources, r)
	}

	if ut.IsEmpty(resources) {
		return nil, ut.NewInfo("empty")
	}

	return resources, nil
}

func getResourcesByIds(db *sql.DB, rids []int) ([]ut.Resource, error) {
	placeholders := make([]string, len(rids))
	args := make([]any, len(rids))
	for i, uid := range rids {
		placeholders[i] = "?"
		args[i] = uid
	}
	placeholderStr := strings.Join(placeholders, ",")

	query := fmt.Sprintf(`
	SELECT
      *
    FROM 
      resources 
    WHERE 
      rid IN (%s)`, placeholderStr)

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("error querying db: %v", err)
		return nil, err
	}

	var resources []ut.Resource
	for rows.Next() {
		var r ut.Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		resources = append(resources, r)
	}

	if ut.IsEmpty(resources) {
		return nil, ut.NewInfo("empty")
	}

	return resources, nil
}

func getResourceByName(db *sql.DB, name string) (ut.Resource, error) {
	var resource ut.Resource

	err := db.QueryRow("SELECT * FROM resources WHERE name = ?", name).Scan(resource.PtrFields()...)
	if err != nil {
		log.Printf("error scanning resource: %v", err)
		return resource, err
	}

	return resource, nil
}

func getResourcesByNameLike(db *sql.DB, name string) ([]ut.Resource, error) {
	rows, err := db.Query(`
    SELECT
      	*
    FROM 
      	resources
	WHERE
		name LIKE ?`, "%"+name+"%")
	if err != nil {
		log.Printf("error querying db: %v", err)
		return nil, err
	}
	defer rows.Close()

	var resources []ut.Resource
	for rows.Next() {
		var r ut.Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		resources = append(resources, r)
	}

	if ut.IsEmpty(resources) {
		return nil, ut.NewInfo("empty")
	}

	return resources, nil
}

func deleteResourcesByIds(db *sql.DB, rids []string) (int64, error) {
	// can't have empty arg (might be destructive)
	if len(rids) == 0 {
		log.Printf("empty argument, returning...")
		return 0, fmt.Errorf("must provide input ids")
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

func deleteResourceByName(db *sql.DB, name string) error {
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

func updateResourceNameById(db *sql.DB, rid, name string) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("error starting transaction: %v", err)
		return err
	}

	query := `
    UPDATE 
      resources 
    SET 
      name = ?, updated_at = ?, accessed_at = ?
    WHERE 
      rid = ?;
  `

	res, err := tx.Exec(query, name, ut.CurrentTime(), ut.CurrentTime(), rid)
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

func updateResourcePermsById(db *sql.DB, rid, perms string) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("error starting transaction: %v", err)
		return err
	}

	query := `
    UPDATE 
      resources 
    SET 
      perms = ?, accessed_at = ?, updated_at = ?
    WHERE 
      rid = ?;
  `

	res, err := tx.Exec(query, perms, ut.CurrentTime(), ut.CurrentTime(), rid)
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

func updateResourceOwnerById(db *sql.DB, rid, uid int) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("error starting transaction: %v", err)
		return err
	}

	query := `
    UPDATE 
      resources 
    SET 
      uid = ?, accessed_at = ?, updated_at = ?
    WHERE 
      rid = ?;
  `

	res, err := tx.Exec(query, uid, ut.CurrentTime(), ut.CurrentTime(), rid)
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

func updateResourceGroupById(db *sql.DB, rid, gid int) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("error starting transaction: %v", err)
		return err
	}

	query := `
    UPDATE 
      resources 
    SET 
      gid = ?, accessed_at = ?, updated_at = ?
    WHERE 
      rid = ?;
  `

	res, err := tx.Exec(query, gid, ut.CurrentTime(), ut.CurrentTime(), rid)
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

// updated, better, more inclusive funcs
func getResources(db *sql.DB, sel, table, by, byvalue string, limit int) ([]any, error) {
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
		by_values_split := strings.Split(strings.TrimSpace(byvalue), ",")
		placeholders := make([]string, len(by_values_split))
		for i := range by_values_split {
			placeholders[i] = "?"
		}
		placeholderStr = strings.Join(placeholders, ",")

		query = fmt.Sprintf("%s WHERE %s (%s)", query, by, placeholderStr)
	}

	if limit > 0 {
		limit_str := fmt.Sprintf("LIMIT %d", limit)
		query = fmt.Sprintf("%s %s", query, limit_str)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("error querying db: %v", err)
		return nil, err
	}
	defer rows.Close()

	var resources []any
	for rows.Next() {
		var r ut.Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		resources = append(resources, r)
	}

	return resources, nil
}

func getResource(db *sql.DB, sel, table, by, byvalue string) (any, error) {
	if sel == "" {
		sel = "*"
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", sel, table, by)

	var r ut.Resource
	err := db.QueryRow(query, byvalue).Scan(r.PtrFields()...)
	if err != nil {
		log.Printf("error scanning row: %v", err)
		return r, err
	}

	return r, nil
}

func get[T any](db *sql.DB, sel, table, by, byvalue string, limit int, scanFn func(*sql.Rows) (T, error)) ([]T, error) {
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
		by_values_split := strings.Split(strings.TrimSpace(byvalue), ",")
		placeholders := make([]string, len(by_values_split))
		for i := range by_values_split {
			placeholders[i] = "?"
		}
		placeholderStr = strings.Join(placeholders, ",")

		query = fmt.Sprintf("%s WHERE %s (%s)", query, by, placeholderStr)
	}

	if limit > 0 {
		limit_str := fmt.Sprintf("LIMIT %d", limit)
		query = fmt.Sprintf("%s %s", query, limit_str)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("error querying db: %v", err)
		return nil, err
	}
	defer rows.Close()

	var results []T
	for rows.Next() {
		val, err := scanFn(rows)
		if err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		results = append(results, val)
	}

	return results, nil
}

func scanResource(rows *sql.Rows) (ut.Resource, error) {
	var r ut.Resource
	err := rows.Scan(r.PtrFields()...)
	return r, err
}

func scanVolume(rows *sql.Rows) (ut.Volume, error) {
	var v ut.Volume
	err := rows.Scan(v.PtrFields()...)
	return v, err
}

func scanUserVolume(rows *sql.Rows) (ut.UserVolume, error) {
	var uv ut.UserVolume
	err := rows.Scan(uv.PtrFields()...)
	return uv, err
}

func scanGroupVolume(rows *sql.Rows) (ut.GroupVolume, error) {
	var gv ut.GroupVolume
	err := rows.Scan(gv.PtrFields()...)
	return gv, err
}

func pickScanFn(table string) func(*sql.Rows) (any, error) {
	switch table {
	case "resources":
		return func(rows *sql.Rows) (any, error) {
			return scanResource(rows)
		}
	case "volumes":
		return func(rows *sql.Rows) (any, error) {
			return scanVolume(rows)
		}
	case "userVolume":
		return func(rows *sql.Rows) (any, error) {
			return scanUserVolume(rows)
		}
	case "groupVolume":
		return func(rows *sql.Rows) (any, error) {
			return scanGroupVolume(rows)
		}
	}
	return nil
}
