package utils

/*
	Initialization and general database handling code for the UserspaceAPI
*/

import (
	"database/sql"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

/*
	 In this API we have 4 types of data that we handle

		each data type is handled by a corresponding handler
			- Resources
			- Volumes
			- UserVolumes
			- GroupVolumes
*/

// well this is somewhat suboptimal, since not all db calls can be used from a handler
// but for now we will keep it as is
// should be refactored
// one should choose which database calls he needs for his database connection
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
		log.Fatalf("database path should end with /")
	}
	// check if the database name is valid
	if dbname[len(dbname)-3:] != ".db" {
		log.Fatalf("database name should end with .db")
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
func (m *DBHandler) Init(initSqlArg, database_path, max_open_conns, max_idle_cons, conn_lifetime string) {
	log.Printf("Initializing %v database", m.DBName)
	if err := os.MkdirAll(database_path, 0o740); err != nil {
		log.Fatalf("failed to create directory path %s, destructive: %v", database_path, err)
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
