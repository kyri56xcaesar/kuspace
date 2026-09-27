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

// seedDefaultApps installs the built-in applications that are missing.
// Existing rows (matched by the unique app name) are left untouched, so edits
// made by an admin survive restarts.
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
			INSERT OR IGNORE INTO
				apps (name, image, description, version,
				 author, authorId, status, insertedAt, createdAt)
			VALUES
				(?, ?, ?, ?, ?, ?, ?, ?, ?);
		`, app.FieldsNoID()...)
		if err != nil {
			return fmt.Errorf("failed to install app %q: %w", app.Name, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			installed++
		}
	}
	log.Printf("[USPACE_init] default apps: %d installed, %d already present", installed, len(apps)-installed)

	return nil
}
