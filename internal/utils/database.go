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

	_ "github.com/marcboeker/go-duckdb"
	_ "github.com/mattn/go-sqlite3"
)

type DBHandler struct {
	db        *sql.DB
	db_driver string
	db_path   string
	DBName    string
}

// constructor
func NewDBHandler(dbname, dbpath, db_driver string) DBHandler {

	// perform checks
	// check if the database driver is supported
	if db_driver != "sqlite3" && db_driver != "duckdb" {
		log.Fatalf("unsupported database driver: %s", db_driver)
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

	if err := os.MkdirAll(dbpath, 0755); err != nil {
		log.Fatalf("failed to create the path: %v", err)
	}

	var dbh DBHandler = DBHandler{
		DBName:    dbname,
		db_path:   dbpath,
		db_driver: db_driver,
	}
	return dbh
}

/* must be a pointer since there is no specific connection variable and is depending on the Handler struct*/
func (m *DBHandler) GetConn() (*sql.DB, error) {
	if m.db == nil {
		db, err := sql.Open(m.db_driver, m.db_path+m.DBName)
		if err != nil {
			return nil, err
		}
		m.db = db
	}
	return m.db, nil
}

/* must be a pointer since there is no specific connection variable and is depending on the Handler struct*/
func (m *DBHandler) Close() {
	if m.db != nil {
		m.db.Close()
	}
}

/* Initialization routines for a database */
/* must be a pointer since there is no specific connection variable and is depending on the Handler struct*/
func (m *DBHandler) Init(initSqlArg, max_open_conns, max_idle_cons, conn_lifetime string) {
	log.Printf("Initializing %v database", m.DBName)
	if err := os.MkdirAll(m.db_path, 0o740); err != nil {
		log.Fatalf("failed to create directory path %s, destructive: %v", m.db_path, err)
	}

	/* perform the init db scripts */
	db, err := m.GetConn()
	if err != nil {
		log.Fatalf("couldn't get db connection, destructive: %v", err)
	}
	max_conns, err := strconv.Atoi(max_open_conns)
	if err != nil {
		db.Close()
		log.Fatalf("failed to atoi max_open_conns value: %v", err)
	}
	max_idle, err := strconv.Atoi(max_idle_cons)
	if err != nil {
		db.Close()
		log.Fatalf("failed to atoi max_idle_cons value: %v", err)
	}
	conn_lifetime_int, err := strconv.Atoi(conn_lifetime)
	if err != nil {
		db.Close()
		log.Fatalf("failed to atoi conn_lifetime value: %v", err)
	}
	db.SetMaxOpenConns(max_conns)
	db.SetMaxIdleConns(max_idle)
	db.SetConnMaxLifetime(time.Duration(conn_lifetime_int) * time.Minute)

	// init tables
	_, err = db.Exec(initSqlArg)
	if err != nil {
		db.Close()
		log.Fatalf("failed to init db, destrcutive: %v", err)
	}

}
