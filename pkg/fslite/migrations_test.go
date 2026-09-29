package fslite

import (
	"database/sql"
	"kyri56xcaesar/kuspace/internal/config"
	"path/filepath"
	"testing"
)

// A database from before migrations, holding the damage the start-up fixes
// used to repair: fixed once, recorded, kept.
func TestMigratingAnOldDatabase(t *testing.T) {
	dir := t.TempDir() + "/"
	baseline, err := migrationFiles.ReadFile("migrations/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	old, err := sql.Open("sqlite3", filepath.Join(dir, "fsl.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.ExecContext(t.Context(), string(baseline)+`
		INSERT INTO volumes (vid, name, capacity) VALUES (1, 'vol', 10);
		INSERT INTO resources (rid, uid, gid, vid, vname, size, name) VALUES
			(1, 1001, 1001, 0, 'vol', 5, 'a.txt'),   -- vid 0 and no leading /
			(2, 1001, 1001, 1, 'vol', 7, '/b.txt');  -- fine`); err != nil {
		t.Fatal(err)
	}
	_ = old.Close()

	cfg := config.EnvConfig{
		AuthConfig:   config.AuthConfig{ServiceSecretKey: []byte("s")},
		TokensConfig: config.TokensConfig{JwtSecretKey: []byte("k")},
		FsliteConfig: config.FsliteConfig{
			FslDB: "fsl.db", FslDBPath: dir, FslDBDriver: "sqlite3",
			FslDBMaxOpenConns: "4", FslDBMaxIdleConns: "2", FslDBMaxLifetime: "5",
			FslAccessKey: "admin", FslSecretKey: "adminpassword1",
		},
	}
	open := func() (*FsLite, *sql.DB) {
		fsl := NewFsLite(cfg)
		t.Cleanup(fsl.Close)
		db, err := fsl.dbh.GetConn()
		if err != nil {
			t.Fatal(err)
		}

		return &fsl, db
	}
	fsl, db := open()
	var vid int
	var name string
	if err := db.QueryRowContext(t.Context(), `SELECT vid, name FROM resources WHERE rid = 1`).Scan(&vid, &name); err != nil || vid != 1 || name != "/a.txt" {
		t.Errorf("repaired row: vid %d name %q err %v", vid, name, err)
	}
	var n int
	_ = db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM schema_migrations`).Scan(&n)
	if n != 3 {
		t.Errorf("%d migrations recorded, want 3", n)
	}
	if err := fsl.CheckSpace(t.Context(), 1001, "vol", 1, 100); err != nil {
		t.Errorf("usage views after migrating: %v", err)
	}
	baks, _ := filepath.Glob(dir + "fsl.db.pre-0001-*.bak")
	if len(baks) != 1 {
		t.Errorf("backups: %v", baks)
	}

	// the fixes ran once: damage made now stays (fsck's job, not start-up's)
	if _, err := db.ExecContext(t.Context(), `UPDATE resources SET name = 'c.txt' WHERE rid = 2`); err != nil {
		t.Fatal(err)
	}
	fsl.Close()
	_, db = open()
	if err := db.QueryRowContext(t.Context(), `SELECT name FROM resources WHERE rid = 2`).Scan(&name); err != nil || name != "c.txt" {
		t.Errorf("a finished migration ran again: %q %v", name, err)
	}
}
