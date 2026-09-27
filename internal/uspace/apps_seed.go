package uspace

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

// defaultAppsJSON is the catalog of built-in applications, installed on init.
//
//go:embed applications/applications.json
var defaultAppsJSON []byte

// seedDefaultApps installs the built-in applications that are missing and
// moves rows still pointing at an older built-in image to the current one
// (e.g. -v1 -> -v2). Everything else an admin changed - or a custom image -
// is left untouched.
func (srv *UService) seedDefaultApps(ctx context.Context) error {
	var apps []ut.Application
	if err := json.Unmarshal(defaultAppsJSON, &apps); err != nil {
		return fmt.Errorf("failed to parse default apps: %w", err)
	}

	db, err := srv.jdbh.GetConn()
	if err != nil {
		return fmt.Errorf("failed to retrieve db conn: %w", err)
	}

	installed := 0
	for _, app := range apps {
		now := ut.CurrentTime()
		app.InsertedAt, app.CreatedAt = now, now
		res, err := db.ExecContext(ctx, `
			INSERT INTO
				apps (name, image, description, version,
				 author, authorId, status, insertedAt, createdAt)
			VALUES
				(?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET image = excluded.image, version = excluded.version
				WHERE apps.image LIKE 'kyri56xcaesar/kuspace:applications-%'
				  AND apps.image != excluded.image;
		`, app.FieldsNoID()...)
		if err != nil {
			return fmt.Errorf("failed to install app %q: %w", app.Name, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			installed++
		}
	}
	log.Printf("[USPACE_init] default apps: %d installed or updated, %d unchanged", installed, len(apps)-installed)

	return nil
}
