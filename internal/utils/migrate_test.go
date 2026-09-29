package utils

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func testDB(t *testing.T) (*DBHandler, *sql.DB) {
	t.Helper()
	h := NewDBHandler("m.db", t.TempDir()+"/", "sqlite3")
	h.Init(t.Context(), "", "4", "2", "5")
	t.Cleanup(h.Close)
	db, err := h.GetConn()
	if err != nil {
		t.Fatal(err)
	}

	return &h, db
}

// the jobs table as it was before migrations: no engine column yet
const oldSchema = `CREATE TABLE IF NOT EXISTS jobs (jid INTEGER PRIMARY KEY, uid INTEGER);`

func testMigrations(extra ...Migration) []Migration {
	return append([]Migration{
		{Version: 1, Name: "init", SQL: oldSchema},
		{Version: 2, Name: "job_engine", Func: AddColumns("jobs", Column{"engine", "TEXT"})},
	}, extra...)
}

func columns(t *testing.T, db *sql.DB, table string) string {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var cols []string
	for rows.Next() {
		var c string
		_ = rows.Scan(&c)
		cols = append(cols, c)
	}

	return strings.Join(cols, ",")
}

func recorded(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), `SELECT version || ':' || name FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		out = append(out, s)
	}

	return strings.Join(out, " ")
}

func TestMigrateFreshDatabase(t *testing.T) {
	h, db := testDB(t)
	ctx := t.Context()
	if err := h.Migrate(ctx, testMigrations()); err != nil {
		t.Fatal(err)
	}
	if got := columns(t, db, "jobs"); got != "jid,uid,engine" {
		t.Errorf("columns = %s", got)
	}
	if got := recorded(t, db); got != "1:init 2:job_engine" {
		t.Errorf("recorded = %s", got)
	}
	// nothing to back up in a new database
	if baks, _ := filepath.Glob(filepath.Join(filepath.Dir(h.dbPath+h.DBName), "*.bak")); len(baks) != 0 {
		t.Errorf("backups of a new database: %v", baks)
	}
	// running again changes nothing
	if err := h.Migrate(ctx, testMigrations()); err != nil {
		t.Fatal(err)
	}
	if got := recorded(t, db); got != "1:init 2:job_engine" {
		t.Errorf("after a second run: %s", got)
	}
}

// A database from before migrations: the baseline is recorded without
// changing anything, the later migrations run, the data stays, and a backup
// is taken first.
func TestMigrateBaselinesAnExistingDatabase(t *testing.T) {
	h, db := testDB(t)
	ctx := t.Context()
	if _, err := db.ExecContext(t.Context(), oldSchema+`INSERT INTO jobs (jid, uid) VALUES (7, 1001);`); err != nil {
		t.Fatal(err)
	}
	if err := h.Migrate(ctx, testMigrations()); err != nil {
		t.Fatal(err)
	}
	var uid int
	var engine sql.NullString
	if err := db.QueryRowContext(t.Context(), `SELECT uid, engine FROM jobs WHERE jid = 7`).Scan(&uid, &engine); err != nil || uid != 1001 || engine.Valid {
		t.Errorf("the job: uid %d engine %v err %v", uid, engine, err)
	}
	baks, _ := filepath.Glob(h.dbPath + h.DBName + ".pre-0001-*.bak")
	if len(baks) != 1 {
		t.Fatalf("backups: %v", baks)
	}
	bak, err := sql.Open("sqlite3", baks[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = bak.Close() }()
	if got := columns(t, bak, "jobs"); got != "jid,uid" {
		t.Errorf("the backup holds %s, want the schema before migrating", got)
	}

	// the ad-hoc start-up code had already added the column: still fine
	h2, db2 := testDB(t)
	if _, err := db2.ExecContext(t.Context(), oldSchema+`ALTER TABLE jobs ADD COLUMN engine TEXT;`); err != nil {
		t.Fatal(err)
	}
	if err := h2.Migrate(ctx, testMigrations()); err != nil {
		t.Fatal(err)
	}
	if got := columns(t, db2, "jobs"); got != "jid,uid,engine" {
		t.Errorf("columns = %s", got)
	}
}

func TestFailedMigrationIsRolledBackAndStops(t *testing.T) {
	h, db := testDB(t)
	boom := errors.New("boom")
	migs := testMigrations(
		Migration{Version: 3, Name: "half_done", Func: func(ctx context.Context, tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `CREATE TABLE half (x INTEGER)`); err != nil {
				return err
			}

			return boom
		}},
		Migration{Version: 4, Name: "after", SQL: `CREATE TABLE after (x INTEGER)`},
	)
	if err := h.Migrate(t.Context(), migs); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if got := recorded(t, db); got != "1:init 2:job_engine" {
		t.Errorf("recorded = %s", got)
	}
	for _, table := range []string{"half", "after"} {
		if columns(t, db, table) != "" {
			t.Errorf("table %s exists", table)
		}
	}
}

func TestMigrateRefusesWhatItCantTrust(t *testing.T) {
	t.Run("newer database", func(t *testing.T) {
		h, _ := testDB(t)
		if err := h.Migrate(t.Context(), testMigrations(Migration{Version: 3, Name: "later", SQL: `SELECT 1`})); err != nil {
			t.Fatal(err)
		}
		if err := h.Migrate(t.Context(), testMigrations()); !errors.Is(err, ErrSchemaNewer) {
			t.Errorf("err = %v, want ErrSchemaNewer", err)
		}
	})
	t.Run("renamed migration", func(t *testing.T) {
		h, _ := testDB(t)
		if err := h.Migrate(t.Context(), testMigrations()); err != nil {
			t.Fatal(err)
		}
		migs := testMigrations()
		migs[1].Name = "something_else"
		if err := h.Migrate(t.Context(), migs); !errors.Is(err, ErrInvalid) {
			t.Errorf("err = %v, want ErrInvalid", err)
		}
	})
	t.Run("gap filled later", func(t *testing.T) {
		_, db := testDB(t)
		if _, err := db.ExecContext(t.Context(), migrationsTable); err != nil {
			t.Fatal(err)
		}
		// 1 and 3 ran; 2 appears afterwards
		if _, err := db.ExecContext(t.Context(), `INSERT INTO schema_migrations VALUES (1, 'init', 'x'), (3, 'three', 'x')`); err != nil {
			t.Fatal(err)
		}
		migs := testMigrations(Migration{Version: 3, Name: "three", SQL: `SELECT 1`})
		if _, err := PendingMigrations(t.Context(), db, migs); !errors.Is(err, ErrInvalid) {
			t.Errorf("err = %v, want ErrInvalid", err)
		}
	})
}

func TestLoadMigrations(t *testing.T) {
	fsys := fstest.MapFS{
		"m/0001_init.sql":  {Data: []byte(oldSchema)},
		"m/0003_index.sql": {Data: []byte(`CREATE INDEX i ON jobs(uid)`)},
	}
	migs, err := LoadMigrations(fsys, "m", Migration{Version: 2, Name: "job_engine", Func: AddColumns("jobs", Column{"engine", "TEXT"})})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, m := range migs {
		names = append(names, m.Name)
	}
	if strings.Join(names, ",") != "init,job_engine,index" {
		t.Errorf("order: %v", names)
	}

	for name, fsys := range map[string]fstest.MapFS{
		"gap":      {"m/0001_a.sql": {Data: []byte("SELECT 1")}, "m/0003_c.sql": {Data: []byte("SELECT 1")}},
		"repeat":   {"m/0001_a.sql": {Data: []byte("SELECT 1")}, "m/0001_b.sql": {Data: []byte("SELECT 1")}},
		"bad name": {"m/1_a.sql": {Data: []byte("SELECT 1")}},
		"empty":    {"m/0001_a.sql": {Data: []byte("")}},
	} {
		if _, err := LoadMigrations(fsys, "m"); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}
