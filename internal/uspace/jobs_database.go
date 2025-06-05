package uspace

/*
	database call handlers for "jobs"
	"jobs.db"

	all crud operations

	can be improved

	@used by the api
*/

import (
	"database/sql"
	"fmt"
	ut "kyri56xcaesar/kuspace/internal/utils"
	"log"
	"strings"
	"time"
)

const (
	initSQLJobs = `
	CREATE TABLE IF NOT EXISTS jobs (
		jid INTEGER PRIMARY KEY,
		uid INTEGER,
		description TEXT,
		duration FLOAT,
		input TEXT,
		input_format TEXT,
		output TEXT,
		output_format TEXT,
		logic TEXT,
		logic_body TEXT,
		logic_headers TEXT,
		parameters TEXT,
		status TEXT,
		completed BOOLEAN,
		completed_at DATETIME,
		createdAt DATETIME,
		parallelism INTEGER,
		priority INTEGER,
		memory_request TEXT,
		cpu_request TEXT,
		memory_limit TEXT,
		cpu_limit TEXT,
		ephimeral_storage_request TEXT,
		ephimeral_storage_limit TEXT
	);
	CREATE TABLE IF NOT EXISTS apps (
		id INTEGER PRIMARY KEY,
		name TEXT,
		image TEXT,
		description TEXT,
		version TEXT,
		author TEXT,
		author_id INTEGER,
		status TEXT,
		insertedAt DATETIME,
		createdAt DATETIME
	);
	CREATE SEQUENCE IF NOT EXISTS seq_jobid START 1;
	CREATE SEQUENCE IF NOT EXISTS seq_appid START 1;
`
)

func (srv *UService) insertJob(jb ut.Job) (int64, error) {
	// log.Printf("inserting job in db: %+v", jb)
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return -1, err
	}

	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
	query := `
		INSERT INTO 
			jobs (jid, uid, description, duration, input, input_format, output, output_format, logic, logic_body, logic_headers, parameters, status, completed, createdAt, parallelism, priority, memory_request, cpu_request, memory_limit, cpu_limit, ephimeral_storage_request, ephimeral_storage_limit)
		VALUES
			(nextval('seq_jobid'), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING (jid);
	`

	var jid int64
	err = db.QueryRow(query, jb.UID, jb.Description, jb.Duration, jb.Input, jb.InputFormat, jb.Output, jb.OutputFormat, jb.Logic, jb.LogicBody, jb.LogicHeaders, strings.Join(jb.Params, ","), "pending", jb.Completed, currentTime, jb.Parallelism, jb.Priority, jb.MemoryRequest, jb.CPURequest, jb.MemoryLimit, jb.CPULimit, jb.EphimeralStorageRequest, jb.EphimeralStorageLimit).Scan(&jid)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		return -1, err
	}

	// log.Printf("[Database] Inserted job id: %v", jid)

	// perhaps we require the id, but letgo for now
	return jid, nil
}

// should user an appender
func (srv *UService) insertJobs(jobs []ut.Job) error {

	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}
	query := `
		INSERT INTO 
			jobs (jid, uid, description, duration, input, input_format, output, output_format, logic, logic_body, logic_headers, parameters, status, completed, createdAt, parallelism, priority, memory_request, cpu_request, memory_limit, cpu_limit, ephimeral_storage_request, ephimeral_storage_limit)
		VALUES
			(nextval('seq_jobid'), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING (jid);
	`

	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)
		return err
	}

	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("failed to prepare statement: %v", err)
		return err
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()

	for i := range jobs {
		jb := &(jobs)[i]

		currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
		var jid int64
		err = db.QueryRow(query, jb.UID, jb.Description, jb.Duration, jb.Input, jb.InputFormat, jb.Output, jb.OutputFormat, jb.Logic, jb.LogicBody, jb.LogicHeaders, strings.Join(jb.Params, ","), "pending", jb.Completed, currentTime).Scan(&jid)
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				return err
			}
			log.Printf("failed to execute statement: %v", err)
			return fmt.Errorf("failed to execute insertion query: %v", err)
		}
		jb.Jid = jid
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func (srv *UService) removeJob(jid int) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve db connection: %v", err)
		return err
	}
	query := `
		DELETE FROM
			jobs
		WHERE
			jid = ?
	`
	_, err = db.Exec(query, jid)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		return err
	}
	return nil
}

func (srv *UService) removeJobs(jids []int) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}
	query := `
		DELETE FROM
			jobs
		WHERE
			jid = ?
	`
	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)
		return err
	}
	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("failed to prepare statement: %v", err)
		return err
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()
	for _, jid := range jids {
		_, err := stmt.Exec(jid)
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				return err
			}
			log.Printf("failed to execute statement: %v", err)
			return fmt.Errorf("failed to execute delete query: %v", err)
		}
	}
	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}
	return nil
}

func (srv *UService) getJobByID(jid int) (ut.Job, error) {
	var job ut.Job
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return job, err
	}
	query := `
		SELECT
			*
		FROM
			jobs
		WHERE
			jid = ?
	`
	var params string
	var completedAt, createdAt sql.NullString

	err = db.QueryRow(query, jid).Scan(&job.Jid, &job.UID, &job.Description, &job.Duration, &job.Input, &job.InputFormat, &job.Output, &job.OutputFormat, &job.Logic, &job.LogicBody, &job.LogicHeaders, &params, &job.Status, &job.Completed, &completedAt, &createdAt, &job.Parallelism, &job.Priority, &job.MemoryRequest, &job.CPURequest, &job.MemoryLimit, &job.CPULimit, &job.EphimeralStorageRequest, &job.EphimeralStorageLimit)
	if err != nil {
		log.Printf("failed to query row: %v", err)
		return job, err
	}

	if completedAt.Valid {
		job.CompletedAt = completedAt.String
	} else {
		job.CompletedAt = ""
	}
	if createdAt.Valid {
		job.CreatedAt = createdAt.String
	} else {
		job.CreatedAt = ""
	}

	job.Params = strings.Split(strings.TrimSpace(params), ",")

	return job, nil
}

func (srv *UService) getJobsByUID(uid int) ([]ut.Job, error) {
	var jobs []ut.Job
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return nil, err
	}
	query := `
		SELECT
			*
		FROM
			jobs
		WHERE
			uid = ?
	`
	rows, err := db.Query(query, uid)
	if err != nil {
		log.Printf("failed to query row: %v", err)
		return nil, err
	}

	var (
		params                 string
		completedAt, createdAt sql.NullString
	)
	for rows.Next() {
		var job ut.Job

		err = rows.Scan(&job.Jid, &job.UID, &job.Description, &job.Duration, &job.Input, &job.InputFormat, &job.Output, &job.OutputFormat, &job.Logic, &job.LogicBody, &job.LogicHeaders, &params, &job.Status, &job.Completed, &completedAt, &createdAt, &job.Parallelism, &job.Priority, &job.MemoryRequest, &job.CPURequest, &job.MemoryLimit, &job.CPULimit, &job.EphimeralStorageRequest, &job.EphimeralStorageLimit)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}
		if completedAt.Valid {
			job.CompletedAt = completedAt.String
		} else {
			job.CompletedAt = ""
		}
		if createdAt.Valid {
			job.CreatedAt = createdAt.String
		} else {
			job.CreatedAt = ""
		}

		job.Params = strings.Split(strings.TrimSpace(params), ",")
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (srv *UService) getJobsByUids(uids []int) ([]ut.Job, error) {
	var jobs []ut.Job

	if len(uids) == 0 {
		return jobs, nil // no users, return empty list
	}

	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return nil, err
	}

	// Build placeholders like (?, ?, ?)
	placeholders := make([]string, len(uids))
	args := make([]any, len(uids))
	for i, uid := range uids {
		placeholders[i] = "?"
		args[i] = uid
	}
	placeholderStr := strings.Join(placeholders, ",")

	query := fmt.Sprintf(`
		SELECT
			*
		FROM
			jobs
		WHERE
			uid IN (%s)
	`, placeholderStr)

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("failed to query row: %v", err)
		return nil, err
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var (
		params                 string
		completedAt, createdAt sql.NullString
	)
	for rows.Next() {
		var job ut.Job

		err = rows.Scan(&job.Jid, &job.UID, &job.Description, &job.Duration, &job.Input, &job.InputFormat, &job.Output, &job.OutputFormat, &job.Logic, &job.LogicBody, &job.LogicHeaders, &params, &job.Status, &job.Completed, &completedAt, &createdAt, &job.Parallelism, &job.Priority, &job.MemoryRequest, &job.CPURequest, &job.MemoryLimit, &job.CPULimit, &job.EphimeralStorageRequest, &job.EphimeralStorageLimit)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}
		if completedAt.Valid {
			job.CompletedAt = completedAt.String
		} else {
			job.CompletedAt = ""
		}
		if createdAt.Valid {
			job.CreatedAt = createdAt.String
		} else {
			job.CreatedAt = ""
		}
		job.Params = strings.Split(strings.TrimSpace(params), ",")

		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (srv *UService) getAllJobs(limit, offset string) ([]ut.Job, error) {
	var jobs []ut.Job
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return nil, err
	}
	var query string

	if limit == "" {
		query = `
		SELECT
			*
		FROM
			jobs
	`
	} else if offset == "" {
		query = `
		SELECT
			*
		FROM
			jobs
		LIMIT ?;
			`
	} else {
		query = `
		SELECT
			*
		FROM
			jobs
		LIMIT ? OFFSET ?;
	`
	}

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		log.Printf("failed to query row: %v", err)
		return nil, err
	}

	var (
		params                 string
		completedAt, createdAt sql.NullString
	)
	for rows.Next() {
		var job ut.Job
		err = rows.Scan(&job.Jid, &job.UID, &job.Description, &job.Duration, &job.Input, &job.InputFormat, &job.Output, &job.OutputFormat, &job.Logic, &job.LogicBody, &job.LogicHeaders, &params, &job.Status, &job.Completed, &completedAt, &createdAt, &job.Parallelism, &job.Priority, &job.MemoryRequest, &job.CPURequest, &job.MemoryLimit, &job.CPULimit, &job.EphimeralStorageRequest, &job.EphimeralStorageLimit)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}
		if completedAt.Valid {
			job.CompletedAt = completedAt.String
		} else {
			job.CompletedAt = ""
		}
		if createdAt.Valid {
			job.CreatedAt = createdAt.String
		} else {
			job.CreatedAt = ""
		}
		job.Params = strings.Split(strings.TrimSpace(params), ",")

		jobs = append(jobs, job)

	}
	return jobs, nil
}

func (srv *UService) updateJob(jb ut.Job) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	query := `
		UPDATE jobs
		SET
			description = ?, uid = ?, status = ?, completed = ?
		WHERE
			jid = ?
	`
	_, err = db.Exec(query, jb.Description, jb.UID, jb.Status, jb.Completed, jb.Jid)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		return err
	}

	return nil
}

func (srv *UService) markJobStatus(jid int64, status string, duration time.Duration) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}
	var (
		completed   bool
		currentTime string
	)

	var query string

	if status == "completed" {
		completed = true
		currentTime = time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
		query = `
		UPDATE jobs
		SET
			status = ?, completed = ?, completed_at = ?, duration = ?
		WHERE
			jid = ?
	`
		_, err = db.Exec(query, status, completed, currentTime, duration, jid)
		if err != nil {
			log.Printf("failed to execute query: %v", err)
			return err
		}

	} else {
		query = `
		UPDATE jobs
		SET
			status = ?, completed = ?, duration = ?
		WHERE
			jid = ?
	`
		_, err = db.Exec(query, status, completed, duration, jid)
		if err != nil {
			log.Printf("failed to execute query: %v", err)
			return err
		}

	}

	return nil
}
