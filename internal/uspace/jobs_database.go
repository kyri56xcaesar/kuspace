package uspace

/*
	database call handlers for "jobs"
	"jobs.db"

	all crud operations

	can be improved

	@used by the api
*/

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	ut "kyri56xcaesar/kuspace/internal/utils"
	"log"
	"strings"
	"time"
)

//go:embed migrations/jobs/*.sql
var jobsMigrationFiles embed.FS

// jobsMigrations are jobs.db's migrations (see ut.Migration).
func jobsMigrations() ([]ut.Migration, error) {
	return ut.LoadMigrations(jobsMigrationFiles, "migrations/jobs",
		// added by start-up code before migrations existed: some databases have them
		ut.Migration{Version: 2, Name: "job_engine_and_gid", Func: ut.AddColumns("jobs",
			ut.Column{Name: "engine", Type: "TEXT"}, ut.Column{Name: "gid", Type: "INTEGER"})},
	)
}

// openJobsDB opens jobs.db and brings it up to date.
func openJobsDB(ctx context.Context, h *ut.DBHandler, maxOpen, maxIdle, lifetime string) error {
	h.Init(ctx, "", maxOpen, maxIdle, lifetime)
	migs, err := jobsMigrations()
	if err != nil {
		return err
	}

	return h.Migrate(ctx, migs)
}

func (srv *UService) insertJob(ctx context.Context, jb ut.Job) (int64, error) {
	// log.Printf("inserting job in db: %+v", jb)
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return -1, fmt.Errorf("failed to retrieve db conn: %w", err)
	}

	query := `
		INSERT INTO 
			jobs (uid, description, duration, input, inputFormat, output, outputFormat, logic, logicBody,
			 logicHeaders, parameters, status, completed, createdAt, parallelism, priority, memoryRequest, cpuRequest,
			  memoryLimit, cpuLimit, ephemeralStorageRequest, ephemeralStorageLimit, gid)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING (jid);`

	var jid int64
	err = db.QueryRowContext(ctx, query, jb.UID, jb.Description, jb.Duration, jb.Input,
		jb.InputFormat, jb.Output, jb.OutputFormat, jb.Logic, jb.LogicBody,
		jb.LogicHeaders, strings.Join(jb.Params, ","), "pending", jb.Completed,
		ut.CurrentTime(), jb.Parallelism, jb.Priority, jb.MemoryRequest, jb.CPURequest,
		jb.MemoryLimit, jb.CPULimit, jb.EphemeralStorageRequest, jb.EphemeralStorageLimit, jb.GID).Scan(&jid)
	if err != nil {
		log.Printf("failed to execute query: %v", err)

		return -1, fmt.Errorf("failed to execute query: %w", err)
	}

	if verbose {
		log.Printf("[Database] Inserted job id: %v", jid)
	}

	return jid, nil
}

func (srv *UService) removeJob(ctx context.Context, jid int) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve db connection: %v", err)

		return fmt.Errorf("failed to retrieve db conn: %w", err)
	}
	query := `
		DELETE FROM
			jobs
		WHERE
			jid = ?`
	_, err = db.ExecContext(ctx, query, jid)
	if err != nil {
		log.Printf("failed to execute query: %v", err)

		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

func (srv *UService) removeJobs(ctx context.Context, jids []int) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return fmt.Errorf("failed to retrieve db conn: %w", err)
	}
	query := `
		DELETE FROM
			jobs
		WHERE
			jid = ?`
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		log.Printf("failed to prepare statement: %v", err)

		return fmt.Errorf("failed to prepare transaction statement: %w", err)
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			log.Printf("failed to close statement: %v", err)
		}
	}()
	for _, jid := range jids {
		_, err := stmt.ExecContext(ctx, jid)
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				return err
			}
			log.Printf("failed to execute statement: %v", err)

			return fmt.Errorf("failed to execute delete query: %w", err)
		}
	}
	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (srv *UService) getJobByID(ctx context.Context, jid int) (ut.Job, error) {
	var job ut.Job
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return job, fmt.Errorf("failed to retrieve db conn: %w", err)
	}
	query := `
		SELECT ` + jobColumns + ` FROM jobs
		WHERE
			jid = ?`
	var params string
	var completedAt, createdAt sql.NullString

	err = db.QueryRowContext(ctx, query, jid).Scan(&job.JID, &job.UID, &job.Description, &job.Duration, &job.Input,
		&job.InputFormat, &job.Output, &job.OutputFormat, &job.Logic, &job.LogicBody, &job.LogicHeaders,
		&params, &job.Status, &job.Completed, &completedAt, &createdAt, &job.Parallelism, &job.Priority,
		&job.MemoryRequest, &job.CPURequest, &job.MemoryLimit, &job.CPULimit, &job.EphemeralStorageRequest,
		&job.EphemeralStorageLimit, &job.Engine, &job.GID,
		&job.StatusReason, &job.Attempts)
	if err != nil {
		log.Printf("failed to query row: %v", err)

		return job, fmt.Errorf("failed to query row: %w", err)
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

func (srv *UService) getJobsByUID(ctx context.Context, uid int) ([]ut.Job, error) {
	var jobs []ut.Job
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return nil, fmt.Errorf("failed to retrieve db conn: %w", err)
	}
	query := `
		SELECT ` + jobColumns + ` FROM jobs
		WHERE
			uid = ?`
	rows, err := db.QueryContext(ctx, query, uid)
	if err != nil {
		log.Printf("failed to query row: %v", err)

		return nil, fmt.Errorf("failed to query row: %w", err)
	}

	var (
		params                 string
		completedAt, createdAt sql.NullString
	)
	for rows.Next() {
		var job ut.Job

		err = rows.Scan(&job.JID, &job.UID, &job.Description, &job.Duration, &job.Input,
			&job.InputFormat, &job.Output, &job.OutputFormat, &job.Logic, &job.LogicBody,
			&job.LogicHeaders, &params, &job.Status, &job.Completed, &completedAt, &createdAt,
			&job.Parallelism, &job.Priority, &job.MemoryRequest, &job.CPURequest, &job.MemoryLimit,
			&job.CPULimit, &job.EphemeralStorageRequest, &job.EphemeralStorageLimit, &job.Engine, &job.GID,
			&job.StatusReason, &job.Attempts)
		if err != nil {
			log.Printf("failed to scan row: %v", err)

			return nil, fmt.Errorf("failed to scan row: %w", err)
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

func (srv *UService) getJobsByUIDs(ctx context.Context, uids []int) ([]ut.Job, error) {
	var jobs []ut.Job

	if len(uids) == 0 {
		return jobs, nil // no users, empty list
	}

	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return nil, fmt.Errorf("failed to retrieve db conn: %w", err)
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
		SELECT `+jobColumns+` FROM jobs
		WHERE
			uid IN (%s)`,
		placeholderStr)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Printf("failed to query row: %v", err)

		return nil, fmt.Errorf("failed to execute query: %w", err)
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

		err = rows.Scan(&job.JID, &job.UID, &job.Description, &job.Duration, &job.Input,
			&job.InputFormat, &job.Output, &job.OutputFormat, &job.Logic, &job.LogicBody,
			&job.LogicHeaders, &params, &job.Status, &job.Completed, &completedAt, &createdAt,
			&job.Parallelism, &job.Priority, &job.MemoryRequest, &job.CPURequest, &job.MemoryLimit,
			&job.CPULimit, &job.EphemeralStorageRequest, &job.EphemeralStorageLimit, &job.Engine, &job.GID,
			&job.StatusReason, &job.Attempts)
		if err != nil {
			log.Printf("failed to scan row: %v", err)

			return nil, fmt.Errorf("failed to scan row: %w", err)
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

func (srv *UService) getAllJobs(ctx context.Context, limit, offset string) ([]ut.Job, error) {
	var jobs []ut.Job
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return nil, fmt.Errorf("failed to retrieve db conn: %w", err)
	}
	var query string

	if limit == "" {
		query = `
		SELECT ` + jobColumns + ` FROM jobs`
	} else if offset == "" {
		query = `
		SELECT ` + jobColumns + ` FROM jobs
		LIMIT ?;`
	} else {
		query = `
		SELECT ` + jobColumns + ` FROM jobs
		LIMIT ? OFFSET ?;`
	}

	rows, err := db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		log.Printf("failed to query row: %v", err)

		return nil, fmt.Errorf("failed to query row: %w", err)
	}

	var (
		params                 string
		completedAt, createdAt sql.NullString
	)
	for rows.Next() {
		var job ut.Job
		err = rows.Scan(&job.JID, &job.UID, &job.Description, &job.Duration, &job.Input, &job.InputFormat,
			&job.Output, &job.OutputFormat, &job.Logic, &job.LogicBody, &job.LogicHeaders, &params, &job.Status,
			&job.Completed, &completedAt, &createdAt, &job.Parallelism, &job.Priority, &job.MemoryRequest, &job.CPURequest,
			&job.MemoryLimit, &job.CPULimit, &job.EphemeralStorageRequest, &job.EphemeralStorageLimit, &job.Engine, &job.GID,
			&job.StatusReason, &job.Attempts)
		if err != nil {
			log.Printf("failed to scan row: %v", err)

			return nil, fmt.Errorf("failed to scan row: %w", err)
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

func (srv *UService) updateJob(ctx context.Context, jb ut.Job) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return fmt.Errorf("failed to retrieve db conn: %w", err)
	}

	query := `
		UPDATE jobs
		SET
			description = ?, uid = ?, status = ?, completed = ?
		WHERE
			jid = ?
	`
	_, err = db.ExecContext(ctx, query, jb.Description, jb.UID, jb.Status, jb.Completed, jb.JID)
	if err != nil {
		log.Printf("failed to execute query: %v", err)

		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

func (srv *UService) markJobStatus(ctx context.Context, jid int64, status string, duration time.Duration) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		log.Printf("failed to get database connection: %v", err)

		return fmt.Errorf("failed to retrieve db conn: %w", err)
	}
	var (
		completed bool
		query     string
	)

	if status == "completed" {
		completed = true
		query = `
		UPDATE jobs
		SET
			status = ?, completed = ?, completedAt = ?, duration = ?, lease_until = NULL
		WHERE
			jid = ?
	`
		_, err = db.ExecContext(ctx, query, status, completed, ut.CurrentTime(), duration, jid)
		if err != nil {
			log.Printf("failed to execute query: %v", err)

			return fmt.Errorf("failed to execute query: %w", err)
		}
	} else {
		// running: remember when (the database dispatcher's reaper needs it);
		// a final state ends the job's lease
		query = `
		UPDATE jobs
		SET
			status = ?1, completed = ?2, duration = ?3,
			started_unix = CASE WHEN ?1 = 'running' THEN COALESCE(started_unix, unixepoch()) ELSE started_unix END,
			lease_until = CASE WHEN ?1 IN ('failed', 'cancelled', 'unknown') THEN NULL ELSE lease_until END
		WHERE
			jid = ?4
	`
		_, err = db.ExecContext(ctx, query, status, completed, duration, jid)
		if err != nil {
			log.Printf("failed to execute query: %v", err)

			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

// saveJobLog stores (replaces) the kept output of a job.
func (srv *UService) saveJobLog(ctx context.Context, jid int64, text string) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO job_logs (jid, log, updatedAt) VALUES (?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET log = excluded.log, updatedAt = excluded.updatedAt`,
		jid, text, ut.CurrentTime())

	return err
}

// getJobLog returns the kept output of a job ("" if none was saved).
func (srv *UService) getJobLog(ctx context.Context, jid int64) (string, error) {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		return "", err
	}
	var text string
	err = db.QueryRowContext(ctx, `SELECT log FROM job_logs WHERE jid = ?`, jid).Scan(&text)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}

	return text, err
}

// jobColumns lists the jobs columns in the order the job scans expect
// (explicit instead of SELECT *, so adding a column can't shift them).
const jobColumns = `jid, uid, description, duration, input, inputFormat, output, outputFormat,
	logic, logicBody, logicHeaders, parameters, status, completed, completedAt, createdAt,
	parallelism, priority, memoryRequest, cpuRequest, memoryLimit, cpuLimit,
	ephemeralStorageRequest, ephemeralStorageLimit, COALESCE(engine, ''), COALESCE(gid, 0),
	COALESCE(status_reason, ''), attempts`

// jobsByEngine counts the jobs per recorded engine ("" = none recorded).
func (srv *UService) jobsByEngine(ctx context.Context) (map[string]int, error) {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT COALESCE(engine, ''), COUNT(*) FROM jobs GROUP BY 1`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int{}
	for rows.Next() {
		var engine string
		var n int
		if err := rows.Scan(&engine, &n); err != nil {
			return nil, err
		}
		out[engine] = n
	}

	return out, rows.Err()
}

// setJobReason records why a job is in its state.
func (srv *UService) setJobReason(ctx context.Context, jid int64, reason string) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE jobs SET status_reason = ? WHERE jid = ?`, reason, jid)

	return err
}

// markJobEngine records where a job runs.
func (srv *UService) markJobEngine(ctx context.Context, jid int64, engine string) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE jobs SET engine = ? WHERE jid = ?`, engine, jid)

	return err
}
