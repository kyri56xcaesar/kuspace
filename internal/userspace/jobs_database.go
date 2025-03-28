package userspace

/*
	database call handlers for "jobs"
	"jobs.db"

	@used by the api
*/

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// feels wierd, lastid inserted doesnt work...
func (dbh *DBHandler) InsertJob(jb Job) (int64, error) {
	// log.Printf("inserting job in db: %+v", jb)
	db, err := dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return -1, err
	}

	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
	query := `
		INSERT INTO 
			jobs (jid, uid, description, duration, input, input_format, output, output_format, logic, logic_body, logic_headers, parameters, status, completed, created_at)
		VALUES
			(nextval('seq_jobid'), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING (jid);
	`

	var jid int64
	err = db.QueryRow(query, jb.Uid, jb.Description, jb.Duration, strings.Join(jb.Input, ","), jb.InputFormat, jb.Output, jb.OutputFormat, jb.Logic, jb.LogicBody, jb.LogicHeaders, strings.Join(jb.Params, ","), "pending", jb.Completed, currentTime).Scan(&jid)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		return -1, err
	}

	log.Printf("last inserted id: %v", jid)

	// perhaps we require the id, but letgo for now
	return jid, nil
}

// should user an appender
func (dbh *DBHandler) InsertJobs(jbs []Job) error {
	db, err := dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}
	query := `
		INSERT INTO 
			jobs (jid, uid, description, duration, input, input_format, output, output_format, logic, logic_body, logic_headers, parameters, status, completed, created_at)
		VALUES
			(nextval('seq_jobid'), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
	defer stmt.Close()

	for _, jb := range jbs {
		currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
		var jid int64
		err = db.QueryRow(query, jb.Uid, jb.Description, jb.Duration, strings.Join(jb.Input, ","), jb.InputFormat, jb.Output, jb.OutputFormat, jb.Logic, jb.LogicBody, jb.LogicHeaders, strings.Join(jb.Params, ","), "pending", jb.Completed, currentTime).Scan(&jid)
		if err != nil {
			tx.Rollback()
			log.Printf("failed to execute statement: %v", err)
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func (dbh *DBHandler) RemoveJob(jid int) error {
	db, err := dbh.getConn()
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

func (dbh *DBHandler) RemoveJobs(jids []int) error {
	db, err := dbh.getConn()
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
	defer stmt.Close()
	for _, jid := range jids {
		_, err := stmt.Exec(jid)
		if err != nil {
			tx.Rollback()
			log.Printf("failed to execute statement: %v", err)
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return err
	}
	return nil
}

func (dbh *DBHandler) GetJobById(jid int) (Job, error) {
	var job Job
	db, err := dbh.getConn()
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
	var input, params string
	var cmplted_at sql.NullString

	err = db.QueryRow(query, jid).Scan(&job.Jid, &job.Uid, &job.Description, &job.Duration, &input, &job.InputFormat, &job.Output, &job.OutputFormat, &job.Logic, &job.LogicBody, &job.LogicHeaders, &params, &job.Status, &job.Completed, &job.Created_at, &cmplted_at)
	if err != nil {
		log.Printf("failed to query row: %v", err)
		return job, err
	}

	if cmplted_at.Valid {
		job.Completed_at = cmplted_at.String
	} else {
		job.Completed_at = ""
	}
	job.Input = strings.Split(strings.TrimSpace(input), ",")
	job.Params = strings.Split(strings.TrimSpace(params), ",")

	return job, nil
}

func (dbh *DBHandler) GetJobsByUid(uid int) ([]Job, error) {
	var jobs []Job
	db, err := dbh.getConn()
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
		input, params string
		cmplted_at    sql.NullString
	)
	for rows.Next() {
		var job Job

		err = rows.Scan(&job.Jid, &job.Uid, &job.Description, &job.Duration, &input, &job.InputFormat, &job.Output, &job.OutputFormat, &job.Logic, &job.LogicBody, &job.LogicHeaders, &params, &job.Status, &job.Completed, &job.Created_at, &cmplted_at)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}
		if cmplted_at.Valid {
			job.Completed_at = cmplted_at.String
		} else {
			job.Completed_at = ""
		}
		job.Input = strings.Split(strings.TrimSpace(input), ",")
		job.Params = strings.Split(strings.TrimSpace(params), ",")
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (dbh *DBHandler) GetJobsByUids(uids []int) ([]Job, error) {
	var jobs []Job

	if len(uids) == 0 {
		return jobs, nil // no users, return empty list
	}

	db, err := dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return nil, err
	}

	// Build placeholders like (?, ?, ?)
	placeholders := make([]string, len(uids))
	args := make([]interface{}, len(uids))
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
	defer rows.Close()

	var (
		input, params string
		cmplted_at    sql.NullString
	)
	for rows.Next() {
		var job Job

		err = rows.Scan(&job.Jid, &job.Uid, &job.Description, &job.Duration, &input, &job.InputFormat, &job.Output, &job.OutputFormat, &job.Logic, &job.LogicBody, &job.LogicHeaders, &params, &job.Status, &job.Completed, &job.Created_at, &cmplted_at)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}
		if cmplted_at.Valid {
			job.Completed_at = cmplted_at.String
		} else {
			job.Completed_at = ""
		}
		job.Input = strings.Split(strings.TrimSpace(input), ",")
		job.Params = strings.Split(strings.TrimSpace(params), ",")

		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (dbh *DBHandler) GetAllJobs(limit, offset string) ([]Job, error) {
	var jobs []Job
	db, err := dbh.getConn()
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
		input, params string
		cmplted_at    sql.NullString
	)
	for rows.Next() {
		var job Job
		err = rows.Scan(&job.Jid, &job.Uid, &job.Description, &job.Duration, &input, &job.InputFormat, &job.Output, &job.OutputFormat, &job.Logic, &job.LogicBody, &job.LogicHeaders, &params, &job.Status, &job.Completed, &job.Created_at, &cmplted_at)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}
		if cmplted_at.Valid {
			job.Completed_at = cmplted_at.String
		} else {
			job.Completed_at = ""
		}
		job.Input = strings.Split(strings.TrimSpace(input), ",")
		job.Params = strings.Split(strings.TrimSpace(params), ",")

		jobs = append(jobs, job)

	}
	return jobs, nil
}

func (dbh *DBHandler) UpdateJob(jb Job) error {
	db, err := dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	query := `
		UPDATE jobs
		SET
			uid = ?, input = ?, output = ?, logic = ?, status = ?, completed = ?
		WHERE
			jid = ?
	`
	_, err = db.Exec(query, jb.Uid, jb.Input, jb.Output, jb.Logic, jb.Status, jb.Completed, jb.Jid)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		return err
	}

	return nil
}

func (dbh *DBHandler) MarkStatus(jid int, status string, duration time.Duration) error {
	db, err := dbh.getConn()
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
