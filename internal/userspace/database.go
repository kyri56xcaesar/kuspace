package userspace

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

const (
	USERSPACE_DB_PATH = "data/db/"
	initSql           = `
    	CREATE TABLE IF NOT EXISTS resources (
    	  rid INTEGER PRIMARY KEY,
    	  uid INTEGER,
    	  vid INTEGER,
    	  gid INTEGER,
    	  pid INTEGER,
    	  size BIGINT,
    	  links INTEGER,
    	  perms TEXT,
    	  name TEXT,
    	  type TEXT,
    	  created_at DATETIME,
    	  updated_at DATETIME,
    	  accessed_at DATETIME
    	);
    	CREATE TABLE IF NOT EXISTS volumes (
    	  vid INTEGER PRIMARY KEY,
    	  name TEXT,
    	  path TEXT,
		  dynamic BOOLEAN,
    	  capacity FLOAT,
    	  usage FLOAT
    	);
		CREATE TABLE IF NOT EXISTS userVolume(
			vid INTEGER,
			uid INTEGER,
			usage FLOAT,
			quota FLOAT,
			updated_at DATETIME
		);
  		CREATE TABLE IF NOT EXISTS groupVolume(
  		  vid INTEGER,
  		  gid INTEGER,
  		  usage FLOAT,
  		  quota FLOAT,
  		  updated_at DATETIME
  		);
    	CREATE SEQUENCE IF NOT EXISTS seq_resourceid START 1;
    	CREATE SEQUENCE IF NOT EXISTS seq_volumeid START 1; 
    `
	initSqlJobs = `
		CREATE TABLE IF NOT EXISTS jobs (
			jid INTEGER PRIMARY KEY,
			uid INTEGER,
			input TEXT,
			input_format TEXT,
			output TEXT,
			output_format TEXT,
			logic TEXT,
			logic_body TEXT,
			status TEXT,
			completed BOOLEAN,
			created_at DATETIME,
			completed_at DATETIME
		);
		CREATE SEQUENCE IF NOT EXISTS seq_jobid START 1;
	`
)

var (
	default_capacity    float64
	default_db_path     string
	default_v_path      string
	default_volume_name string = "kUspace_defaultv"
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
	DBName    string
}

// constructor
func NewDBHandler(dbname, db_driver string) DBHandler {

	var dbh DBHandler = DBHandler{
		DBName:    dbname,
		db_driver: db_driver,
	}
	return dbh
}

/* must be a pointer since there is no specific connection variable and is depending on the Handler struct*/
func (m *DBHandler) getConn() (*sql.DB, error) {
	if m.db == nil {
		db, err := sql.Open(m.db_driver, USERSPACE_DB_PATH+m.DBName)
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
	db, err := m.getConn()
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

func (m *DBHandler) InitResourceVolumeSpecific(database_path, volumes_path, capacity string) {
	db, err := m.getConn()
	if err != nil {
		log.Fatalf("couldn't get db connection, destructive: %v", err)
	}
	// specific initialization
	var exists bool
	err = db.QueryRow("SELECT COUNT(*) > 0 FROM volumes").Scan(&exists)
	if err != nil {
		log.Fatalf("failed to scan exists query: %v", err)
	}
	if !exists {
		// insert an init volume
		vquery := `
		INSERT INTO 
			volumes (vid, name, path, dynamic, capacity, usage)
		VALUES
			(nextval('seq_volumeid'), ?, ?, ?, ?, ?)
		 `
		_, err := db.Exec(vquery, default_volume_name, volumes_path, "true", capacity, 0)
		if err != nil {
			log.Fatalf("failed to insert init data, destructive: %v", err)
		}
	}
	default_capacity, err = strconv.ParseFloat(capacity, 64)
	if err != nil {
		log.Fatalf("failed to atoi capacity value: %v", err)
	}
	default_db_path = database_path
	default_v_path = volumes_path

	log.Printf("default capacity: %v", default_capacity)
	log.Printf("default db path: %v", default_db_path)
	log.Printf("default v path: %v", default_v_path)

	// insert root user/group volume claims(should be everything...)
	// or perhaps avoid it? idk
	err = db.QueryRow("SELECT COUNT(*) > 0 FROM userVolume").Scan(&exists)
	if err != nil {
		log.Fatalf("failed to scan exists query: %v", err)
	}

	if !exists {
		// insert an init volume
		vquery := `
		INSERT INTO 
			userVolume (vid, uid, usage, quota, updated_at)
		VALUES
			(?, ?, ?, ?, ?)
		 `

		currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")

		_, err = db.Exec(vquery, 1, 0, 0, capacity, currentTime)
		if err != nil {
			log.Fatalf("failed to insert init data, destructive: %v", err)
		}
	}

	err = db.QueryRow("SELECT COUNT(*) > 0 FROM groupVolume").Scan(&exists)
	if err != nil {
		log.Fatalf("failed to scan exists query: %v", err)
	}

	if !exists {
		// insert an init volume
		vquery := `
		INSERT INTO 
			groupVolume (vid, gid, usage, quota, updated_at)
		VALUES
			(?, ?, ?, ?, ?),
    	(?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?)

		 `

		currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")

		res, err := db.Exec(vquery, 1, 0, 0, capacity, currentTime, 1, 100, 0, capacity, currentTime, 1, 1000, 0, capacity, currentTime)
		if err != nil {
			log.Fatalf("failed to insert init data, destructive: %v", err)
		}

		rAff, err := res.RowsAffected()
		if err != nil {
			log.Printf("failed to retrieve rows affected: %v", err)
			return
		}

		if rAff != 3 {
			log.Fatalf("failed to insert essential groupVolumes")
		}
	}
}
