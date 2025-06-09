package uspace

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

func (srv *UService) insertApp(app ut.Application) (int64, error) {
	// log.Printf("inserting job in db: %+v", jb)
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return -1, fmt.Errorf("failed to retrieve db conn: %w", err)
	}

	currentTime := ut.CurrentTime()
	app.InsertedAt = currentTime

	_, err = time.Parse("2006-01-02 15:04:05-07:00", app.CreatedAt)
	if err != nil {
		app.CreatedAt = currentTime
	}

	query := `
		INSERT INTO 
			apps (id, name, image, description, version,
			 author, authorId, status, insertedAt, createdAt)
		VALUES
			(nextval('seq_appid'), ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING (id);
	`

	var id int64
	err = db.QueryRow(query, app.FieldsNoID()...).Scan(&id)
	if err != nil {
		log.Printf("failed to execute query: %v", err)

		return -1, fmt.Errorf("failed to execute query: %w", err)
	}
	if verbose {
		log.Printf("[Database] Inserted job id: %v", id)
	}

	return id, nil
}

// should user an appender
func (srv *UService) insertApps(apps []ut.Application) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return fmt.Errorf("failed to retrieve db conn: %w", err)
	}
	query := `
		INSERT INTO 
			apps (id, name, image, description, version,
			 author, authorId, status, insertedAt, createdAt)
		VALUES
			(nextval('seq_appid'), ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING (id);`

	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("failed to prepare statement: %v", err)

		return fmt.Errorf("failed to prepare transaction: %w", err)
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()

	currentTime := ut.CurrentTime()
	for i := range apps {
		app := &(apps)[i]

		app.InsertedAt = currentTime
		_, err = time.Parse("2006-01-02 15:04:05-07:00", app.CreatedAt)
		if err != nil {
			app.CreatedAt = currentTime
		}
		var id int64
		err = db.QueryRow(query, app.FieldsNoID()...).Scan(&id)
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				return err
			}
			log.Printf("failed to execute statement: %v", err)

			return fmt.Errorf("failed to execute insertion rolling back: %w", err)
		}
		app.ID = id
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (srv *UService) removeApp(id int) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve db connection: %v", err)

		return fmt.Errorf("failed to retrieve db conn: %w", err)
	}
	query := `
		DELETE FROM
			apps
		WHERE
			id = ?`
	_, err = db.Exec(query, id)
	if err != nil {
		log.Printf("failed to execute query: %v", err)

		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

func (srv *UService) removeApps(ids []int) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return fmt.Errorf("failed to retrieve db conn: %w", err)
	}
	query := `
		DELETE FROM
			apps
		WHERE
			id = ?`
	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("failed to prepare statement: %v", err)

		return fmt.Errorf("failed to prepare transaction statement: %w", err)
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()
	for _, id := range ids {
		_, err := stmt.Exec(id)
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				return err
			}
			log.Printf("failed to execute statement: %v", err)

			return fmt.Errorf("failed to execute deletion: %w", err)
		}
	}
	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (srv *UService) getAppByID(id int) (ut.Application, error) {
	var app ut.Application
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return app, fmt.Errorf("failed to retrieve db conn: %w", err)
	}
	query := `
		SELECT
			*
		FROM
			apps
		WHERE
			id = ?`

	var insertedAt, createdAt sql.NullString
	err = db.QueryRow(query, id).Scan(&app.ID, &app.Name, &app.Image, &app.Description,
		&app.Version, &app.Author, &app.AuthorID, &app.Status, &insertedAt, &createdAt)
	if err != nil {
		log.Printf("failed to query row: %v", err)

		return app, fmt.Errorf("failed to query row: %w", err)
	}

	if insertedAt.Valid {
		app.InsertedAt = insertedAt.String
	} else {
		app.InsertedAt = ""
	}
	if createdAt.Valid {
		app.CreatedAt = createdAt.String
	} else {
		app.CreatedAt = ""
	}

	return app, nil
}

func (srv *UService) getAppByNameAndVersion(name, version string) (ut.Application, error) {
	var app ut.Application
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return app, fmt.Errorf("failed to retrieve db conn: %w", err)
	}
	query := `
		SELECT
			*
		FROM
			apps
		WHERE
			name = ? AND version = ?
	`
	var insertedAt, createdAt sql.NullString
	err = db.QueryRow(query, name, version).Scan(&app.ID, &app.Name, &app.Image, &app.Description,
		&app.Version, &app.Author, &app.AuthorID, &app.Status, &insertedAt, &createdAt)
	if err != nil {
		log.Printf("failed to query row: %v", err)

		return app, fmt.Errorf("failed to query row: %w", err)
	}

	if insertedAt.Valid {
		app.InsertedAt = insertedAt.String
	} else {
		app.InsertedAt = ""
	}
	if createdAt.Valid {
		app.CreatedAt = createdAt.String
	} else {
		app.CreatedAt = ""
	}

	return app, nil
}

func (srv *UService) getAppsByIDs(ids []int) ([]ut.Application, error) {
	var apps []ut.Application

	if len(ids) == 0 {
		return apps, nil
	}

	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return nil, fmt.Errorf("failed to retrieve db conn: %w", err)
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, uid := range ids {
		placeholders[i] = "?"
		args[i] = uid
	}
	placeholderStr := strings.Join(placeholders, ",")

	query := fmt.Sprintf(`
		SELECT
			*
		FROM
			apps
		WHERE
			id IN (%s)
	`, placeholderStr)

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("failed to query row: %v", err)

		return nil, fmt.Errorf("failed to query row: %w", err)
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var insertedAt, createdAt sql.NullString

	for rows.Next() {
		var app ut.Application

		err = rows.Scan(&app.ID, &app.Name, &app.Image, &app.Description, &app.Version,
			&app.Author, &app.AuthorID, &app.Status, &insertedAt, &createdAt)

		if err != nil {
			log.Printf("failed to scan row: %v", err)

			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		if insertedAt.Valid {
			app.InsertedAt = insertedAt.String
		} else {
			app.InsertedAt = ""
		}
		if createdAt.Valid {
			app.CreatedAt = createdAt.String
		} else {
			app.CreatedAt = ""
		}

		apps = append(apps, app)
	}

	return apps, nil
}

func (srv *UService) getAllApps(limit, offset string) ([]ut.Application, error) {
	var apps []ut.Application
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return nil, fmt.Errorf("failed to retrieve db conn: %w", err)
	}
	var query string

	if limit == "" {
		query = `
		SELECT
			*
		FROM
			apps`
	} else if offset == "" {
		query = `
		SELECT
			*
		FROM
			apps
		LIMIT ?;`
	} else {
		query = `
		SELECT
			*
		FROM
			apps
		LIMIT ? OFFSET ?;`
	}

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		log.Printf("failed to query row: %v", err)

		return nil, fmt.Errorf("failed to query row: %w", err)
	}

	var insertedAt, createdAt sql.NullString

	for rows.Next() {
		var app ut.Application
		err = rows.Scan(&app.ID, &app.Name, &app.Image, &app.Description, &app.Version,
			&app.Author, &app.AuthorID, &app.Status, &insertedAt, &createdAt)
		if err != nil {
			log.Printf("failed to scan row: %v", err)

			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		if insertedAt.Valid {
			app.InsertedAt = insertedAt.String
		} else {
			app.InsertedAt = ""
		}
		if createdAt.Valid {
			app.CreatedAt = createdAt.String
		} else {
			app.CreatedAt = ""
		}

		apps = append(apps, app)
	}

	return apps, nil
}

func (srv *UService) updateApp(a ut.Application) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return fmt.Errorf("failed to retrieve db conn: %w", err)
	}

	query := `
		UPDATE apps
		SET
			name = ?, image = ?, description = ?, version = ?,
			 author = ?, authorId = ?, status = ?, insertedAt = ?, createdAt = ? 
		WHERE
			id = ?
	`
	_, err = db.Exec(query, a.Name, a.Image, a.Description,
		a.Version, a.Author, a.AuthorID, a.Status, a.InsertedAt, a.CreatedAt, a.ID)
	if err != nil {
		log.Printf("failed to execute query: %v", err)

		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}
