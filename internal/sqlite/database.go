// Package sqlite opens the services' SQLite databases and migrates them
// (moved out of utils).
//
// This file defines the DBHandler struct and associated methods for managing SQL database connections
// using the SQLite3 driver. It includes routines for creating and validating
// database paths, opening and closing connections, and initializing database schemas.
//
// Types:
//   - DBHandler: Encapsulates database connection details and provides methods for connection management.
//
// Functions:
//   - NewDBHandler: Constructs a new DBHandler, validating driver, path, and database name.
//   - (*DBHandler) GetConn: Lazily opens and returns a database connection.
//   - (*DBHandler) Close: Closes the database connection if open.
//   - (*DBHandler) Init: Initializes the database, sets connection pool parameters, and executes
//     initialization SQL scripts.
//
// Usage:
//
//	Use NewDBHandler to create a handler, then call Init to set up the database schema and connection pool.
//	Use GetConn to obtain a connection for queries, and Close to release resources when done.
//
// Supported driver: "sqlite3". (The DuckDB driver was dropped: nothing
// used it, and its bundled C++ library went into every binary.)
//
// Database driver
package sqlite

/*
	initialization and general database handling code
*/

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	// the Driver for sqlite3
	_ "github.com/mattn/go-sqlite3"
)

// DBHandler encapsulates needed information for a database driver
//
// Methods:
//
//	{
//			init, close
//	}
type DBHandler struct {
	db       *sql.DB
	dbDriver string
	dbPath   string
	DBName   string
}

// NewDBHandler a constructor of a DBHandler
func NewDBHandler(dbname, dbpath, dbDriver string) DBHandler {
	// perform checks
	// check if the database driver is supported
	if dbDriver != "sqlite3" {
		log.Fatalf("unsupported database driver: %s", dbDriver)
	}

	inMemory := dbname == ":memory:" || dbpath == ":memory:" || dbpath == "" && dbname == ""
	if !inMemory {
		// check if the database path is valid
		if dbpath == "" {
			log.Fatalf("database path is empty")
		}
		// check if the database name is valid
		if dbname == "" {
			log.Fatalf("database name is empty")
		}
		// check if the database path is valid
		if dbpath[len(dbpath)-1:] != "/" {
			log.Printf("database path %q should end with /... fixing...", dbpath)
			dbpath += "/"
		}
		// check if the database name is valid
		if dbname[len(dbname)-3:] != ".db" {
			log.Fatalf("database name %q should end with .db", dbname)
		}

		if err := os.MkdirAll(dbpath, 0o755); err != nil {
			log.Fatalf("failed to create the path: %v", err)
		}
	}

	return DBHandler{
		DBName:   dbname,
		dbPath:   dbpath,
		dbDriver: dbDriver,
	}
}

// GetConn as a method returns the actual db Driver, if it doesn't exist it instatiates it
/* must be a pointer since there is no specific connection variable and is depending on the Handler struct*/
// "Singleton"
func (m *DBHandler) GetConn() (*sql.DB, error) {
	if m.db == nil {
		var dsn string
		if m.DBName == ":memory:" || m.dbPath == ":memory:" {
			dsn = ":memory:"
		} else {
			dsn = m.dbPath + m.DBName
			if m.dbDriver == "sqlite3" {
				// wait on locks instead of failing with "database is locked", let
				// readers run alongside the writer, and enforce the FOREIGN KEY ...
				// ON DELETE CASCADE the schemas declare (SQLite ignores them unless
				// enabled per connection).
				dsn += "?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on"
			}
		}

		db, err := sql.Open(m.dbDriver, dsn)
		if err != nil {
			log.Printf("Failed to connect to %s: %v", m.dbDriver, err)

			return nil, err
		}
		m.db = db
	}

	return m.db, nil
}

// Close method applies close call to the database
/* must be a pointer since there is no specific connection variable and is depending on the Handler struct*/
func (m *DBHandler) Close() {
	if m.db != nil {
		err := m.db.Close()
		if err != nil {
			log.Printf("failed to close the database connection... should be fatal..: %v", err)
		}
	}
}

// Init method applies all needed logic on startup of the db
/* Initialization routines for a database */
/* must be a pointer since there is no specific connection variable and is depending on the Handler struct*/
func (m *DBHandler) Init(ctx context.Context, initSQLarg, maxOpenConns, maxIdleCons, connLifetime string) {
	log.Printf("Initializing %v database", m.DBName)
	if err := os.MkdirAll(m.dbPath, 0o740); err != nil {
		log.Fatalf("failed to create directory path %s, destructive: %v", m.dbPath, err)
	}

	/* perform the init db scripts */
	db, err := m.GetConn()
	if err != nil {
		log.Fatalf("couldn't get db connection, destructive: %v", err)
	}
	maxConns, err := strconv.Atoi(maxOpenConns)
	if err != nil {
		m.Close()
		log.Fatalf("failed to atoi max_open_conns value: %v", err)
	}
	maxIdle, err := strconv.Atoi(maxIdleCons)
	if err != nil {
		m.Close()
		log.Fatalf("failed to atoi max_idle_cons value: %v", err)
	}
	connLifetimeInt, err := strconv.Atoi(connLifetime)
	if err != nil {
		m.Close()
		log.Fatalf("failed to atoi conn_lifetime value: %v", err)
	}
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(time.Duration(connLifetimeInt) * time.Minute)

	// init tables (databases with migrations pass "" and call Migrate)
	if initSQLarg == "" {
		return
	}
	_, err = db.ExecContext(ctx, initSQLarg)
	if err != nil {
		m.Close()
		log.Fatalf("failed to init db, destrcutive: %v", err)
	}
}

// Migrate brings the database up to date with migs (see migrate.go). An
// existing SQLite file is first copied to <name>.pre-NNNN-<time>.bak, NNNN
// being the first pending migration.
func (m *DBHandler) Migrate(ctx context.Context, migs []Migration) error {
	db, err := m.GetConn()
	if err != nil {
		return err
	}
	pending, err := PendingMigrations(ctx, db, migs)
	if err != nil || len(pending) == 0 {
		return err
	}
	if err := m.backup(ctx, db, pending[0].Version); err != nil {
		return fmt.Errorf("backup before migrating: %w", err)
	}
	applied, err := ApplyMigrations(ctx, db, migs)
	for _, a := range applied {
		log.Printf("[%s] applied migration %04d_%s", m.DBName, a.Version, a.Name)
	}

	return err
}

// backup copies an SQLite database that holds tables (a new, empty one has
// nothing to lose) with VACUUM INTO, which gives a consistent copy of a
// database in WAL mode.
func (m *DBHandler) backup(ctx context.Context, db *sql.DB, version int) error {
	if m.dbDriver != "sqlite3" || m.DBName == ":memory:" || m.dbPath == ":memory:" {
		return nil
	}
	var tables int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'
		AND name NOT IN ('schema_migrations', 'sqlite_sequence')`).Scan(&tables); err != nil {
		return err
	}
	if tables == 0 {
		return nil
	}
	dst := fmt.Sprintf("%s%s.pre-%04d-%s.bak", m.dbPath, m.DBName, version, time.Now().UTC().Format("20060102T150405"))
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, dst); err != nil {
		return err
	}
	log.Printf("[%s] backed up to %s before migrating", m.DBName, dst)

	return nil
}
