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
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/mattn/go-sqlite3"
	"log"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

/* database call handlers regarding the Resource table */

// NormalizeName returns a resource name in its one stored form: a single
// leading "/" (object storage treats "/x" and "x" as the same object, so the
// metadata must not keep both).
func NormalizeName(name string) string {
	return "/" + strings.TrimLeft(name, "/")
}

// repairVolumeIDs points rows whose vid names no existing volume (uploads
// used to record vid 0) at the volume their vname refers to.
func repairVolumeIDs(ctx context.Context, db *sql.DB) (int64, error) {
	res, err := db.ExecContext(ctx, `UPDATE resources SET vid = (SELECT v.vid FROM volumes v WHERE v.name = resources.vname)
		WHERE vid NOT IN (SELECT vid FROM volumes) AND vname IN (SELECT name FROM volumes)`)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

// normalizeStoredNames rewrites names stored without the leading "/" (older
// rows) and returns how many changed.
func normalizeStoredNames(ctx context.Context, db *sql.DB) (int64, error) {
	res, err := db.ExecContext(ctx, `UPDATE resources SET name = '/' || name WHERE name NOT LIKE '/%'`)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

func insertResource(ctx context.Context, db querier, resource ut.Resource) error {
	resource.Name = NormalizeName(resource.Name)
	query := `
    INSERT INTO 
      resources (uid, gid, vid, vname, size, links, perms, name, path, type, createdAt, updatedAt, accessedAt)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ? ,? ,?, ?, ?);  
	`
	currentTime := ut.CurrentTime()
	resource.AccessedAt = currentTime
	resource.CreatedAt = currentTime
	resource.UpdatedAt = currentTime
	_, err := db.ExecContext(ctx, query, resource.FieldsNoID()...)
	if isUniqueViolation(err) {
		return fmt.Errorf("%w: %s in %s", ErrResourceExists, resource.Name, resource.Vname)
	}
	if err != nil {
		log.Printf("[FSL_DB_insRes] failed to insert the resource: %v", err)

		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// Errors fslite returns; each wraps one of the shared kinds in internal/utils
// (ut.ErrExists, ut.ErrNotFound, ...), so callers can match either.
var (
	// ErrResourceExists is returned when a resource with that name already
	// exists in the volume (enforced by the unique (vname, name) index).
	ErrResourceExists = fmt.Errorf("resource %w", ut.ErrExists)
	// ErrResourceNotFound is returned when no resource has that name in the volume.
	ErrResourceNotFound = fmt.Errorf("resource %w", ut.ErrNotFound)
	// ErrVolumeExists is returned when a volume with that name already exists.
	ErrVolumeExists = fmt.Errorf("volume %w", ut.ErrExists)
	// ErrVolumeNotFound is returned when no volume has that name or id.
	ErrVolumeNotFound = fmt.Errorf("volume %w", ut.ErrNotFound)
)

// isUniqueViolation reports a unique/primary-key conflict: by SQLite's
// error code, or - for the DuckDB driver, which has no typed error - its
// message.
func isUniqueViolation(err error) bool {
	var se sqlite3.Error
	if errors.As(err, &se) {
		return se.ExtendedCode == sqlite3.ErrConstraintUnique || se.ExtendedCode == sqlite3.ErrConstraintPrimaryKey
	}

	return err != nil && strings.Contains(err.Error(), "Duplicate key")
}

// ensureUniqueNames adds the unique (vname, name) index that makes duplicate
// names impossible even for concurrent uploads (a check-then-insert can't).
// On a database that already holds duplicates it reports them instead.
func ensureUniqueNames(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_resources_vname_name ON resources(vname, name)`)
	if err == nil {
		return nil
	}
	var dups int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (SELECT 1 FROM resources GROUP BY vname, name HAVING COUNT(*) > 1)`).Scan(&dups)

	return fmt.Errorf("%d duplicated (volume, name) pair(s) prevent the unique index: %w", dups, err)
}

func insertResources(ctx context.Context, db *sql.DB, resources []ut.Resource) error {
	for i := range resources {
		resources[i].Name = NormalizeName(resources[i].Name)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[FSL_DB_insRess] failed to begin transacation: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `
    INSERT INTO 
      resources (uid, gid, vid, vname, size, links, perms, name, path, type, createdAt, updatedAt, accessedAt)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ? ,? ,?, ?, ?);
	`

	stmt, err := tx.PrepareContext(ctx, query)
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
		_, err = stmt.ExecContext(ctx, r.FieldsNoID()...)
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

func getAllResources(ctx context.Context, db *sql.DB) ([]ut.Resource, error) {
	rows, err := db.QueryContext(ctx, `
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

	if resources == nil {
		resources = []ut.Resource{}
	}

	return resources, nil
}

func getResourcesByIDs(ctx context.Context, db *sql.DB, rids []int) ([]ut.Resource, error) {
	args := make([]any, len(rids))
	for i, uid := range rids {
		args[i] = uid
	}

	query := fmt.Sprintf("SELECT * FROM resources WHERE rid IN (%s)", strings.TrimRight(strings.Repeat("?,", len(rids)), ","))

	rows, err := db.QueryContext(ctx, query, args...)
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

	if resources == nil {
		resources = []ut.Resource{}
	}

	return resources, nil
}

func getResourceByNameAndVolume(ctx context.Context, db *sql.DB, name, volume string) (ut.Resource, error) {
	var resource ut.Resource

	err := db.QueryRowContext(ctx, "SELECT * FROM resources WHERE name = ? AND vname = ? LIMIT 1", name, volume).
		Scan(resource.PtrFields()...)
	if err != nil {
		log.Printf("[FSL_DB_getResByNameVol] error scanning resource: %v", err)

		return resource, fmt.Errorf("failed to scan row: %w", err)
	}

	return resource, nil
}

func exists(ctx context.Context, db *sql.DB, name, volume string) (bool, error) {
	var dummy int
	err := db.QueryRowContext(ctx, `
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

// getResourcesByPrefix lists the resources whose name starts with
// "/"+prefix, in one volume (every volume when vname is ""). It is an exact,
// case-sensitive comparison: it used to match "%name%" anywhere in the name
// across all volumes, and LIKE is case-insensitive (fuzzing found "AB"
// listing "/ab") and treats "_" and "%" as wildcards.
func getResourcesByPrefix(ctx context.Context, db *sql.DB, vname, prefix string) ([]ut.Resource, error) {
	rows, err := db.QueryContext(ctx, `
    SELECT * FROM resources
    WHERE substr(name, 1, length(?1)) = ?1 AND (?2 = '' OR vname = ?2)
    ORDER BY name`, NormalizeName(prefix), vname)
	if err != nil {
		log.Printf("[FSL_DB_getResByPrefix] error querying db: %v", err)

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
			log.Printf("[FSL_DB_getResByPrefix] error scanning row: %v", err)

			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		resources = append(resources, r)
	}

	if err = rows.Err(); err != nil {
		log.Printf("[FSL_DB_getResByPrefix] row iteration error: %v", err)

		return nil, fmt.Errorf("iteration error: %w", err)
	}
	if resources == nil {
		resources = []ut.Resource{}
	}

	return resources, nil
}

func deleteResourceByNameAndVolume(ctx context.Context, db *sql.DB, name, volume string) error {
	name = NormalizeName(name) // it used to match raw names: "a.txt" deleted nothing and reported success
	res, err := db.ExecContext(ctx, "DELETE FROM resources WHERE name = ? AND vname = ?", name, volume)
	if err != nil {
		return fmt.Errorf("delete resource: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %s in %s", ErrResourceNotFound, name, volume)
	}

	return nil
}

// updateResourceNameAndVolByName renames/moves the resource `name` in
// fromVol to `newname` in vol (vid follows the volume). An empty fromVol
// matches any volume (legacy callers).
func updateResourceNameAndVolByName(ctx context.Context, db *sql.DB, name, newname, vol, fromVol string) error {
	name, newname = NormalizeName(name), NormalizeName(newname)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[FSL_DB_updateResNameVolumeById] error starting transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `
    UPDATE 
      resources 
    SET 
      name = ?, vname = ?, vid = COALESCE((SELECT vid FROM volumes WHERE name = ?), vid),
      updatedAt = ?, accessedAt = ?
    WHERE 
      name = ? AND (? = '' OR vname = ?);
  `

	res, err := tx.ExecContext(ctx, query, newname, vol, vol, ut.CurrentTime(), ut.CurrentTime(), name, fromVol, fromVol)
	if err != nil {
		log.Printf("[FSL_DB_updateResNameVolumeById] error executing query: %v", err)

		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	_, err = res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateResNameVolumeById] failed to get rows affected")

		return fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_updateResNameVolumeById] error committing transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func updateResourcePermsByID(ctx context.Context, db *sql.DB, rid, perms string) error {
	tx, err := db.BeginTx(ctx, nil)
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

	res, err := tx.ExecContext(ctx, query, perms, ut.CurrentTime(), ut.CurrentTime(), rid)
	if err != nil {
		log.Printf("[FSL_DB_updateResPermsById] error executing query: %v", err)

		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	_, err = res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateResPermsById] failed to get rows affected")

		return fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_updateResPermsById] error committing transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func updateResourceOwnerByID(ctx context.Context, db *sql.DB, rid, uid int) error {
	tx, err := db.BeginTx(ctx, nil)
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

	res, err := tx.ExecContext(ctx, query, uid, ut.CurrentTime(), ut.CurrentTime(), rid)
	if err != nil {
		log.Printf("[FSL_DB_updateResOwnerById] error executing query: %v", err)

		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	_, err = res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateResOwnerById] failed to get rows affected")

		return fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_updateResOwnerById] error committing transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func updateResourceGroupByID(ctx context.Context, db *sql.DB, rid, gid int) error {
	tx, err := db.BeginTx(ctx, nil)
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

	res, err := tx.ExecContext(ctx, query, gid, ut.CurrentTime(), ut.CurrentTime(), rid)
	if err != nil {
		log.Printf("[FSL_DB_updateResGroupById] error executing query: %v", err)

		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	_, err = res.RowsAffected()
	if err != nil {
		log.Printf("[FSL_DB_updateResGroupById] failed to get rows affected")

		return fmt.Errorf("failed to retrieve rows affected %w", err)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("[FSL_DB_updateResGroupById] error committing transaction: %v", err)

		return fmt.Errorf("failed to commit  transaction: %w", err)
	}

	return nil
}
