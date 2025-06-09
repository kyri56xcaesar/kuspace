// Package fslite provides database handlers for managing "resources" in the uspace.db database.
// This file contains functions for inserting, querying, updating, and deleting resource records.
// The handlers are used by the API layer to interact with the resources table.
//
// Functions in this file include:
//   - insertResource: Insert a new resource into the database.
//   - insertResourceUniqueName: Insert a new resource only if its name is unique.
//   - insertResources: Batch insert multiple resources.
//   - insertResourcesUniqueName: Batch insert multiple resources, ensuring unique names.
//   - getAllResourcesAt: Retrieve all resources at a specific path.
//   - getAllResources: Retrieve all resources from the database.
//   - getResourcesByIDs: Retrieve resources by a list of resource IDs.
//   - getResourceByName: Retrieve a resource by its name.
//   - getResourcesByNameLike: Retrieve resources with names matching a pattern.
//   - deleteResourcesByIDs: Delete resources by a list of IDs and return total size deleted.
//   - deleteResourceByName: Delete a resource by its name.
//   - updateResourceNameById: Update the name of a resource by its ID.
//   - updateResourcePermsById: Update the permissions of a resource by its ID.
//   - updateResourceOwnerById: Update the owner (UID) of a resource by its ID.
//   - updateResourceGroupById: Update the group (GID) of a resource by its ID.
//   - getResources: Generic resource query with flexible selection and filtering.
//   - getResource: Generic single resource query with flexible selection and filtering.
//   - get: Generic query function with custom scan function for different table types.
//   - scanResource, scanVolume, scanUserVolume, scanGroupVolume: Helper functions to scan rows into structs.
//   - pickScanFn: Returns the appropriate scan function for a given table.
//
// All functions use parameterized queries to prevent SQL injection and handle transactions where appropriate.
// Logging is performed for error handling and debugging purposes.
package fslite

/*
	database call handlers for "resources"
	"uspace.db"

	@used by the api
*/

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

/* database call handlers regarding the Resource table */
func insertResource(db *sql.DB, resource ut.Resource) error {
	query := `
    INSERT INTO 
      resources (rid, uid, gid, vid, vname, size, links, perms, name, path, type, createdAt, updatedAt, accessedAt)
	VALUES (nextval('seqResourceId'), ?, ?, ?, ?, ?, ?, ?, ?, ? ,? ,?, ?, ?);  
	`
	currentTime := ut.CurrentTime()
	resource.AccessedAt = currentTime
	resource.CreatedAt = currentTime
	resource.UpdatedAt = currentTime
	_, err := db.Exec(query, resource.FieldsNoID()...)
	if err != nil {
		log.Printf("[FSL_DB_insRes] failed to insert the resource: %v", err)

		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

func insertResourceUniqueName(db *sql.DB, resource ut.Resource) error {
	// Check if a resource with the same name and UID already exists
	queryCheck := `SELECT 1 FROM resources WHERE name = ? LIMIT 1;`
	var exists int
	err := db.QueryRow(queryCheck, resource.Name).Scan(&exists)

	if err == nil {
		log.Printf("[FSL_DB_insResUnique] resource with name '%s' already exists", resource.Name)

		return fmt.Errorf("resource with name '%s' already exists", resource.Name)
	} else if !errors.Is(err, sql.ErrNoRows) {
		// Return any other query errors
		log.Printf("[FSL_DB_insResUnique] error checking name uniqueness: %v", err)

		return fmt.Errorf("failed to execute query: %w", err)
	}
	currentTime := ut.CurrentTime()
	resource.AccessedAt = currentTime
	resource.CreatedAt = currentTime
	resource.UpdatedAt = currentTime
	// Insert the resource if no duplicate was found
	queryInsert := `
    INSERT INTO 
      resources (rid, uid, gid, vid, vname, size, links, perms, name, path, type, createdAt, updatedAt, accessedAt)
	VALUES (nextval('seqResourceId'), ?, ?, ?, ?, ?, ?, ?, ?, ? ,? ,?, ?, ?);
  `

	_, err = db.Exec(queryInsert, resource.FieldsNoID()...)
	if err != nil {
		log.Printf("[FSL_DB_insResUnique] failed to insert the resource: %v", err)

		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

func insertResources(db *sql.DB, resources []ut.Resource) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_insRess] failed to begin transacation: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `
    INSERT INTO 
      resources (rid, uid, gid, vid, vname, size, links, perms, name, path, type, createdAt, updatedAt, accessedAt)
	VALUES (nextval('seqResourceId'), ?, ?, ?, ?, ?, ?, ?, ?, ? ,? ,?, ?, ?);
	`

	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("[FSL_DB_insRess] error preparing transaction: %v", err)

		return fmt.Errorf("failed to prepare transaction: %w", err)
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()

	currentTime := ut.CurrentTime()
	for _, r := range resources {
		r.AccessedAt = currentTime
		r.CreatedAt = currentTime
		r.UpdatedAt = currentTime
		_, err = stmt.Exec(r.FieldsNoID()...)
		if err != nil {
			log.Printf("[FSL_DB_insRess] error executing transaction: %v", err)

			return fmt.Errorf("failed to execute transaction: %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_insRess] failed to commit transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func insertResourcesUniqueName(db *sql.DB, resources []ut.Resource) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Prepare the SELECT query to check if the resource exists
	queryCheck := `SELECT 1 FROM resources WHERE name = ? LIMIT 1;`
	stmtCheck, err := tx.Prepare(queryCheck)
	if err != nil {
		log.Printf("error preparing uniqueness check statement: %v", err)

		return fmt.Errorf("failed to prepare transaction: %w", err)
	}
	defer func() {
		err := stmtCheck.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()
	// Prepare the INSERT statement
	queryInsert := `
    INSERT INTO 
      resources (rid, uid, gid, vid, vname, size, links, perms, name, path, type, createdAt, updatedAt, accessedAt)
	VALUES (nextval('seqResourceId'), ?, ?, ?, ?, ?, ?, ?, ?, ? ,? ,?, ?, ?);
	`
	stmtInsert, err := tx.Prepare(queryInsert)
	if err != nil {
		log.Printf("error preparing insert statement: %v", err)

		return fmt.Errorf("failed to prepare transaction: %w", err)
	}
	defer func() {
		err := stmtInsert.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()

	for _, r := range resources {
		var exists int

		log.Printf("resource: %+v", r)
		// Check if the resource already exists
		err = stmtCheck.QueryRow(r.Name).Scan(&exists)
		if err == nil {
			log.Printf("resource with name '%s' already exists", r.Name)

			return fmt.Errorf("resource with name '%s' already exists", r.Name)
		} else if !errors.Is(err, sql.ErrNoRows) {
			// If any other error occurs during the query,return it
			log.Printf("error checking name uniqueness: %v", err)

			return fmt.Errorf("failed to execute transaction: %w", err)
		}

		// If the resource doesn't exist, insert it
		currentTime := ut.CurrentTime()
		r.AccessedAt = currentTime
		r.CreatedAt = currentTime
		r.UpdatedAt = currentTime
		_, err = stmtInsert.Exec(r.FieldsNoID()...)
		if err != nil {
			log.Printf("error executing insert: %v", err)

			return fmt.Errorf("failed to execute transaction: %w", err)
		}
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
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
		log.Printf("[FSL_DB_getRess] error querying db: %v", err)

		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var resources []ut.Resource
	for rows.Next() {
		var r ut.Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("[FSL_DB_getRess] failed to scan resource: %v", err)

			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		resources = append(resources, r)
	}

	if err = rows.Err(); err != nil {
		log.Printf("[FSL_DB_getRess] row iteration error: %v", err)

		return nil, fmt.Errorf("iteration error: %w", err)
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
		log.Printf("[FSL_DB_getRess] error querying db: %v", err)

		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var resources []ut.Resource
	for rows.Next() {
		var r ut.Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("[FSL_DB_getRess] error scanning row: %v", err)

			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		resources = append(resources, r)
	}

	if err = rows.Err(); err != nil {
		log.Printf("[FSL_DB_getRess] row iteration error: %v", err)

		return nil, fmt.Errorf("iteration error: %w", err)
	}

	if ut.IsEmpty(resources) {
		return nil, ut.NewInfo("empty")
	}

	return resources, nil
}

func getResourcesByIDs(db *sql.DB, rids []int) ([]ut.Resource, error) {
	placeholders := make([]string, 0, len(rids))
	args := make([]any, 0, len(rids))
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
		log.Printf("[FSL_DB_getResByIds] error querying db: %v", err)

		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("[FSL_DB] failed to close rows: %v", err)
		}
	}()

	var resources []ut.Resource
	for rows.Next() {
		var r ut.Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("[FSL_DB_getResByIds] error scanning row: %v", err)

			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		resources = append(resources, r)
	}
	// check for iteration errors
	if err = rows.Err(); err != nil {
		log.Printf("[FSL_DB_getResByIds] row iteration error: %v", err)

		return nil, fmt.Errorf("iteration error: %w", err)
	}

	if ut.IsEmpty(resources) {
		return nil, ut.NewInfo("empty")
	}

	return resources, nil
}

func getResourcesByName(db *sql.DB, name string) ([]ut.Resource, error) {
	var resources []ut.Resource

	rows, err := db.Query("SELECT * FROM resources WHERE name = ?", name)
	if err != nil {
		log.Printf("[FSL_DB_getResByName] error performing query: %v", err)

		return resources, fmt.Errorf("failed to execute query: %w", err)
	}

	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("[FSL_DB] failed to close rows: %v", err)
		}
	}()

	for rows.Next() {
		var r ut.Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("[FSL_DB_getResByName] failed to scan resource")

			return resources, fmt.Errorf("failed to scan row: %w", err)
		}
		resources = append(resources, r)
	}

	if err = rows.Err(); err != nil {
		log.Printf("[FSL_DB_getResByName] row iteration error: %v", err)

		return nil, fmt.Errorf("iteration error: %w", err)
	}

	if ut.IsEmpty(resources) {
		return nil, ut.NewInfo("empty")
	}

	return resources, nil
}

func getResourceByNameAndVolume(db *sql.DB, name, volume string) (ut.Resource, error) {
	var resource ut.Resource

	err := db.QueryRow("SELECT * FROM resources WHERE name = ? AND vname = ? LIMIT 1", name, volume).
		Scan(resource.PtrFields()...)
	if err != nil {
		log.Printf("[FSL_DB_getResByNameVol] error scanning resource: %v", err)

		return resource, fmt.Errorf("failed to scan row: %w", err)
	}

	return resource, nil
}

func exists(db *sql.DB, name, volume string) (bool, error) {
	var dummy int
	err := db.QueryRow(`
        SELECT 1 FROM resources 
        WHERE name = ? AND vname = ? 
        LIMIT 1
    `, name, volume).Scan(&dummy)

	if err == sql.ErrNoRows {
		return false, nil // does not exist
	}
	if err != nil {
		return false, fmt.Errorf("DB error checking existence: %w", err)
	}

	return true, nil // exists
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
		log.Printf("[FSL_DB_getResByNameLike] error querying db: %v", err)

		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var resources []ut.Resource
	for rows.Next() {
		var r ut.Resource
		err = rows.Scan(r.PtrFields()...)
		if err != nil {
			log.Printf("[FSL_DB_getResByNameLike] error scanning row: %v", err)

			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		resources = append(resources, r)
	}

	if err = rows.Err(); err != nil {
		log.Printf("[FSL_DB_getResByNameLike] row iteration error: %v", err)

		return nil, fmt.Errorf("iteration error: %w", err)
	}
	if ut.IsEmpty(resources) {
		return nil, ut.NewInfo("empty")
	}

	return resources, nil
}

func deleteResourcesByIDs(db *sql.DB, rids []string) (int64, error) {
	// can't have empty arg (might be destructive)
	if len(rids) == 0 {
		log.Printf("[FSL_DB_delResByIds] empty argument, returning...")

		return 0, errors.New("must provide input ids")
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_delResByIds] error starting transaction: %v", err)

		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}

	placeholders := make([]string, 0, len(rids))
	args := make([]any, 0, len(rids))
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
		log.Printf("[FSL_DB_delResByIds] failed to execute query: %v", err)

		return 0, fmt.Errorf("failed to execute transaction: %w", err)
	}

	defer func() {
		err := sres.Close()
		if err != nil {
			log.Printf("[FSL_DB] failed to close rows: %v", err)
		}
	}()

	res, err := tx.Exec(query, args...)
	if err != nil {
		log.Printf("[FSL_DB_delResByIds] failed to execute query: %v", err)

		return 0, fmt.Errorf("failed to execute transaction: %w", err)
	}

	for sres.Next() {
		var currentSize int64
		err = sres.Scan(&currentSize)
		if err != nil {
			log.Printf("[FSL_DB_delResByIds] error scanning size of resource: %v", err)

			return 0, fmt.Errorf("failed to scan row: %w", err)
		}
		size += currentSize
	}
	defer func() {
		err := sres.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_delResByIds] failed to get rows affected: %v", err)

		return 0, fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_delResByIds] failed to commit transaction: %v", err)

		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}
	if verbose {
		log.Printf("[FSL_DB_delResByIds] deleted %v rows", rAff)
	}

	return size, nil
}

func deleteResourceByName(db *sql.DB, name string) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_delResByName] error starting transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	res, err := tx.Exec("DELETE FROM resources WHERE name = ?", name)
	if err != nil {
		log.Printf("[FSL_DB_delResByName] failed to execute query: %v", err)

		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_delResByName] failed to get rows affected")
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_delResByName] failed to commit transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	if verbose {
		log.Printf("[FSL_DB_delResByNameVolume] deleted %v rows", rAff)
	}

	return nil
}

func deleteResourceByNameAndVolume(db *sql.DB, name, volume string) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_delResByNameVolume] error starting transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	res, err := tx.Exec("DELETE FROM resources WHERE name = ? AND vname = ?", name, volume)
	if err != nil {
		log.Printf("[FSL_DB_delResByNameVolume] failed to execute query: %v", err)

		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_delResByNameVolume] failed to get rows affected")
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_delResByNameVolume] failed to commit transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	if verbose {
		log.Printf("[FSL_DB_delResByNameVolume] deleted %v rows", rAff)
	}

	return nil
}

func updateResourceNameByID(db *sql.DB, rid, name string) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_updateResNameById] error starting transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	query := `
    UPDATE 
      resources 
    SET 
      name = ?, updatedAt = ?, accessedAt = ?
    WHERE 
      rid = ?;
  `
	res, err := tx.Exec(query, name, ut.CurrentTime(), ut.CurrentTime(), rid)
	if err != nil {
		log.Printf("[FSL_DB_updateResNameById] error executing query: %v", err)

		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateResNameById] failed to get rows affected")
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_updateResNameById] error committing transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	if verbose {
		log.Printf("[FSL_DB_updateResNameById] updated %v rows", rAff)
	}

	return nil
}

func updateResourceNameAndVolByName(db *sql.DB, name, newname, vol string) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_updateResNameVolumeById] error starting transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `
    UPDATE 
      resources 
    SET 
      name = ?, vname = ?, updatedAt = ?, accessedAt = ?
    WHERE 
      name = ?;
  `

	res, err := tx.Exec(query, newname, vol, ut.CurrentTime(), ut.CurrentTime(), name)
	if err != nil {
		log.Printf("[FSL_DB_updateResNameVolumeById] error executing query: %v", err)

		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateResNameVolumeById] failed to get rows affected")

		return fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_updateResNameVolumeById] error committing transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	if verbose {
		log.Printf("[FSL_DB_updateResNameVolumeById] updated %v rows", rAff)
	}

	return nil
}

func updateResourcePermsByID(db *sql.DB, rid, perms string) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_updateResPermsById] error starting transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `
    UPDATE 
      resources 
    SET 
      perms = ?, accessedAt = ?, updatedAt = ?
    WHERE 
      rid = ?;
  `

	res, err := tx.Exec(query, perms, ut.CurrentTime(), ut.CurrentTime(), rid)
	if err != nil {
		log.Printf("[FSL_DB_updateResPermsById] error executing query: %v", err)

		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateResPermsById] failed to get rows affected")

		return fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_updateResPermsById] error committing transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	if verbose {
		log.Printf("[FSL_DB_updateResPermsById] updated %v rows", rAff)
	}

	return nil
}

func updateResourceOwnerByID(db *sql.DB, rid, uid int) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_updateResOwnerById] error starting transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	query := `
    UPDATE 
      resources 
    SET 
      uid = ?, accessedAt = ?, updatedAt = ?
    WHERE 
      rid = ?;
  `

	res, err := tx.Exec(query, uid, ut.CurrentTime(), ut.CurrentTime(), rid)
	if err != nil {
		log.Printf("[FSL_DB_updateResOwnerById] error executing query: %v", err)

		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateResOwnerById] failed to get rows affected")

		return fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_updateResOwnerById] error committing transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	if verbose {
		log.Printf("[FSL_DB_updateResOwnerById] updated %v rows", rAff)
	}

	return nil
}

func updateResourceGroupByID(db *sql.DB, rid, gid int) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("[FSL_DB_updateResGroupById] error starting transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `
    UPDATE 
      resources 
    SET 
      gid = ?, accessedAt = ?, updatedAt = ?
    WHERE 
      rid = ?;
  `

	res, err := tx.Exec(query, gid, ut.CurrentTime(), ut.CurrentTime(), rid)
	if err != nil {
		log.Printf("[FSL_DB_updateResGroupById] error executing query: %v", err)

		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateResGroupById] failed to get rows affected")

		return fmt.Errorf("failed to retrieve rows affected %w", err)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_updateResGroupById] error committing transaction: %v", err)

		return fmt.Errorf("failed to commit  transaction: %w", err)
	}
	if verbose {
		log.Printf("[FSL_DB_updateResGroupById] updated %v rows", rAff)
	}

	return nil
}
