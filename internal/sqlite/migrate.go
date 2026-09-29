package sqlite

/*
	Schema migrations: each database owns an ordered list of numbered
	migrations (SQL files embedded in its package, plus Go functions for
	data fixes SQL can't express). A schema_migrations table records what
	has run; at start-up the pending ones run in order, each in its own
	transaction, and a failure stops the service.

	Existing databases are baselined: migration 1 is the schema as it was
	before migrations existed, written with IF NOT EXISTS, so on an existing
	database it changes nothing and is only recorded. Migrations are
	forward-only: a bad one is fixed by a new one, never by editing it.
*/

import (
	ut "kyri56xcaesar/kuspace/internal/utils"

	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// Migration is one numbered change to a database: SQL, or Func for what
// SQL can't express (both run in the migration's transaction).
type Migration struct {
	Version int
	Name    string
	SQL     string
	Func    func(ctx context.Context, tx *sql.Tx) error
}

// ErrSchemaNewer means the database has run migrations this build doesn't
// have: it was used by a newer version, and this one must not touch it.
var ErrSchemaNewer = errors.New("the database was migrated by a newer version")

var migrationFile = regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.sql$`)

// LoadMigrations reads the NNNN_name.sql files in dir of fsys, adds the Go
// migrations in funcs and returns them in order. Versions must run 1, 2, 3...
// without gaps or repeats.
func LoadMigrations(fsys fs.FS, dir string, funcs ...Migration) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	migs := append([]Migration{}, funcs...)
	for _, e := range entries {
		m := migrationFile.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migration file %q is not named NNNN_name.sql (%w)", e.Name(), ut.ErrInvalid)
		}
		body, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		version, _ := strconv.Atoi(m[1])
		migs = append(migs, Migration{Version: version, Name: m[2], SQL: string(body)})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].Version < migs[j].Version })
	for i, m := range migs {
		if m.Version != i+1 {
			return nil, fmt.Errorf("migrations must be numbered 1, 2, 3...: found %d at position %d (%w)", m.Version, i+1, ut.ErrInvalid)
		}
		if (m.SQL == "") == (m.Func == nil) {
			return nil, fmt.Errorf("migration %d (%s) needs exactly one of SQL and Func (%w)", m.Version, m.Name, ut.ErrInvalid)
		}
	}

	return migs, nil
}

const migrationsTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at TEXT NOT NULL
)`

// PendingMigrations returns the migrations db hasn't run yet.
func PendingMigrations(ctx context.Context, db *sql.DB, migs []Migration) ([]Migration, error) {
	if _, err := db.ExecContext(ctx, migrationsTable); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT version, name FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	applied := map[int]string{}
	for rows.Next() {
		var v int
		var name string
		if err := rows.Scan(&v, &name); err != nil {
			return nil, err
		}
		applied[v] = name
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	highest := 0
	for v, name := range applied {
		highest = max(highest, v)
		if v > len(migs) {
			return nil, fmt.Errorf("%w: it has migration %d (%s), this build knows %d", ErrSchemaNewer, v, name, len(migs))
		}
		if migs[v-1].Name != name {
			return nil, fmt.Errorf("migration %d ran as %q but is now %q: migrations must not be renamed or replaced (%w)",
				v, name, migs[v-1].Name, ut.ErrInvalid)
		}
	}
	var pending []Migration
	for _, m := range migs {
		if _, ok := applied[m.Version]; !ok {
			pending = append(pending, m)
		}
	}
	if len(pending) > 0 && pending[0].Version < highest {
		return nil, fmt.Errorf("migration %d was added after later ones ran (%w)", pending[0].Version, ut.ErrInvalid)
	}

	return pending, nil
}

// ApplyMigrations runs the pending migrations of db in order, each in its
// own transaction with its schema_migrations row, and returns the ones it
// ran. It stops at the first failure (that migration is rolled back).
func ApplyMigrations(ctx context.Context, db *sql.DB, migs []Migration) ([]Migration, error) {
	pending, err := PendingMigrations(ctx, db, migs)
	if err != nil {
		return nil, err
	}
	for i, m := range pending {
		if err := applyMigration(ctx, db, m); err != nil {
			return pending[:i], fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
		}
	}

	return pending, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if m.Func != nil {
		err = m.Func(ctx, tx)
	} else {
		_, err = tx.ExecContext(ctx, m.SQL)
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		m.Version, m.Name, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}

	return tx.Commit()
}

// Column is a column AddColumns adds: name and SQL type (with any DEFAULT).
type Column struct{ Name, Type string }

// AddColumns returns a migration step adding the columns table lacks
// (SQLite has no ADD COLUMN IF NOT EXISTS). Columns already there are left
// alone: databases where the ad-hoc start-up code added them come through
// unchanged.
func AddColumns(table string, cols ...Column) func(ctx context.Context, tx *sql.Tx) error {
	return func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
		if err != nil {
			return err
		}
		have := map[string]bool{}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				_ = rows.Close()

				return err
			}
			have[name] = true
		}
		_ = rows.Close()
		if len(have) == 0 {
			return fmt.Errorf("no table %q (%w)", table, ut.ErrNotFound)
		}
		for _, c := range cols {
			if have[c.Name] {
				continue
			}
			if _, err := tx.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+c.Name+` `+c.Type); err != nil {
				return err
			}
		}

		return nil
	}
}
