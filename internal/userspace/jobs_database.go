package userspace

import (
	"log"
	"time"
)

type JobDBHandler interface {
	InsertJob(Job) error
	InsertJobs([]Job) error
	RemoveJob(int) error
	RemoveJobs([]int) error
	GetJobById(int) (Job, error)
	GetJobsByUid(int) ([]Job, error)
	GetJobsByUids([]int) ([]Job, error)
	GetAllJobs() ([]Job, error)
	UpdateJob(Job) error
	MarkStatus(int, string) error
	PatchStatus(int, string) error
}

type Jdh struct {
	dbh *DBHandler
}

func (j *Jdh) InsertJob(jb Job) error {
	db, err := j.dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
	query := `
		INSERT INTO 
			jobs (jid, uid, input, output, logic, status, completed, created_at)
		VALUES
			(nextval('seq_jobid'), ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.Exec(query, jb.Uid, jb.Input, jb.Output, jb.Status, currentTime)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		return err
	}

	// perhaps we require the id, but letgo for now
	return nil
}

func (j *Jdh) InsertJobs(jbs []Job) error {
	db, err := j.dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}
	query := `
		INSERT INTO 
			jobs (jid, uid, input, output, logic, status, completed, created_at)
		VALUES
			(nextval('seq_jobid'), ?, ?, ?, ?, ?, ?, ?)
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
		_, err := stmt.Exec(jb.Uid, jb.Input, jb.Output, jb.Logic, jb.Status, jb.Completed, currentTime)
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

func (j *Jdh) RemoveJob(jid int) error {
	db, err := j.dbh.getConn()
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

func (j *Jdh) RemoveJobs(jids []int) error {
	db, err := j.dbh.getConn()
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

func (j *Jdh) GetJobById(jid int) (Job, error) {
	var job Job
	db, err := j.dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return job, err
	}
	query := `
		SELECT
			jid, uid, input, output, logic, status, completed, created_at
		FROM
			jobs
		WHERE
			jid = ?
	`
	err = db.QueryRow(query, jid).Scan(job.PtrFields()...)
	if err != nil {
		log.Printf("failed to query row: %v", err)
		return job, err
	}
	return job, nil
}

func (j *Jdh) GetJobsByUid(uid int) ([]Job, error) {
	var jobs []Job
	db, err := j.dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return nil, err
	}
	query := `
		SELECT
			jid, uid, input, output, logic, status, completed, created_at
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

	for rows.Next() {
		var job Job
		err = rows.Scan(job.PtrFields()...)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (j *Jdh) GetJobsByUids(uids []int) ([]Job, error) {
	var jobs []Job
	db, err := j.dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return nil, err
	}
	query := `
		SELECT
			jid, uid, input, output, logic, status, completed, created_at
		FROM
			jobs
		WHERE
			uid IN (?)
	`
	rows, err := db.Query(query, uids)
	if err != nil {
		log.Printf("failed to query row: %v", err)
		return nil, err
	}

	for rows.Next() {
		var job Job
		err = rows.Scan(job.PtrFields()...)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (j *Jdh) GetAllJobs() ([]Job, error) {
	var jobs []Job
	db, err := j.dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return nil, err
	}
	query := `
		SELECT
			*
		FROM
			jobs

	`
	rows, err := db.Query(query)
	if err != nil {
		log.Printf("failed to query row: %v", err)
		return nil, err
	}

	for rows.Next() {
		var job Job
		err = rows.Scan(job.PtrFields()...)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (j *Jdh) UpdateJob(jb Job) error {
	db, err := j.dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	query := `
		UPDATE
			jobs
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

func (j *Jdh) PatchStatus(jid int, status string) error {
	db, err := j.dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}

	query := `
		UPDATE
			jobs
		SET
			status = ?
		WHERE
			jid = ?
	`
	_, err = db.Exec(query, status, jid)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		return err
	}

	return nil
}

func (j *Jdh) MarkStatus(jid int, status string) error {
	db, err := j.dbh.getConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)
		return err
	}
	var (
		completed   bool
		currentTime string
	)

	if status == "completed" {
		completed = true
		currentTime = time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
	}
	query := `
		UPDATE
			jobs
		SET
			status = ?, completed = ?, completed_at = ?
		WHERE
			jid = ?
	`
	_, err = db.Exec(query, status, completed, currentTime, jid)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		return err
	}

	return nil
}
