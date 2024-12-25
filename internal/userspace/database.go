package userspace

import (
	"database/sql"
	"log"
	"os"
)

const (
	USERSPACE_DB = "data/db/userspace.db"
	initSql      = `
    CREATE TABLE IF NOT EXISTS resources (
      rid INTEGER AUTOINCREMENT,
      uid INTEGER,
      vid INTEGER,
      name TEXT,
      type TEXT,
      pid INTEGER,
      oid INTEGER,
      gid INTEGER,
      perms TEXT,
      size INTEGER,
      created_at DATETIME,
      updated_at DATETIME
    );

    CREATE TABLE IF NOT EXISTS volumes (
      vid INTEGER AUTOINCREMENT,
      path TEXT,
      capacity INTEGER,
      usage INTEGER
    );
    `
)

type DBHandler struct {
	db     *sql.DB
	DBName string
}

func (m *DBHandler) getConn() (*sql.DB, error) {
	if m.db == nil {
		db, err := sql.Open("duckdb", "data/db/"+m.DBName)
		if err != nil {
			return nil, err
		}
		m.db = db
	}
	return m.db, nil
}

func (m *DBHandler) Close() {
	if m.db != nil {
		m.db.Close()
	}
}

func (m *DBHandler) Init() {
	log.Printf("Initializing %v database", m.DBName)
	_, err := os.Stat("data")
	if err != nil {
		err = os.Mkdir("data", 0o700)
		if err != nil {
			panic("failed to make new directory.")
		}
	}

	_, err = os.Stat("data/db")
	if err != nil {
		err = os.Mkdir("data/db", 0o700)
		if err != nil {
			panic("failed to make new directory.")
		}
	}

	db, err := m.getConn()
	if err != nil {
		panic("destructive")
	}

	err = db.Exec(initSql)
	if err != nil {
		panic("failed to init db, destrcutive")
	}
}
