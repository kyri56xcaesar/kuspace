// Package utils provides utility functions and types for database initialization and handling.
//
// This file defines the DBHandler struct and associated methods for managing SQL database connections
// using supported drivers (SQLite3 and DuckDB). It includes routines for creating and validating
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
// Supported Drivers:
//   - "sqlite3"
//   - "duckdb"
//
// Database driver
package utils

/*
	initialization and general database handling code
*/

import (
	"database/sql"
	"log"
	"os"
	"strconv"
	"time"

	// the Driver for duckdb
	_ "github.com/marcboeker/go-duckdb"
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
	if dbDriver != "sqlite3" && dbDriver != "duckdb" {
		log.Fatalf("unsupported database driver: %s", dbDriver)
	}
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

	dbh := DBHandler{
		DBName:   dbname,
		dbPath:   dbpath,
		dbDriver: dbDriver,
	}

	return dbh
}

// GetConn as a method returns the actual db Driver, if it doesn't exist it instatiates it
/* must be a pointer since there is no specific connection variable and is depending on the Handler struct*/
// "Singleton"
func (m *DBHandler) GetConn() (*sql.DB, error) {
	if m.db == nil {
		db, err := sql.Open(m.dbDriver, m.dbPath+m.DBName)
		if err != nil {
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
func (m *DBHandler) Init(initSQLarg, maxOpenConns, maxIdleCons, connLifetime string) {
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

	// init tables
	_, err = db.Exec(initSQLarg)
	if err != nil {
		m.Close()
		log.Fatalf("failed to init db, destrcutive: %v", err)
	}
}
