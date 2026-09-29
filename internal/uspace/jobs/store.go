package jobs

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
	"log"
	"strings"
	"time"

	"kyri56xcaesar/kuspace/internal/model"
	"kyri56xcaesar/kuspace/internal/sqlite"
	ut "kyri56xcaesar/kuspace/internal/utils"
)

//go:embed migrations/*.sql
var jobsMigrationFiles embed.FS

// jobsMigrations are jobs.db's migrations (see sqlite.Migration).
func jobsMigrations() ([]sqlite.Migration, error) {
	return sqlite.LoadMigrations(jobsMigrationFiles, "migrations",
		// added by start-up code before migrations existed: some databases have them
		sqlite.Migration{Version: 2, Name: "job_engine_and_gid", Func: sqlite.AddColumns("jobs",
			sqlite.Column{Name: "engine", Type: "TEXT"}, sqlite.Column{Name: "gid", Type: "INTEGER"})},
	)
}

// Open opens jobs.db and brings it up to date.
// Store is the jobs table (and the job logs and settings kept with it in
// jobs.db). Open the database first.
type Store struct {
	db *sqlite.DBHandler
}

// NewStore reads and writes jobs in db.
func NewStore(db *sqlite.DBHandler) *Store { return &Store{db: db} }

func Open(ctx context.Context, h *sqlite.DBHandler, maxOpen, maxIdle, lifetime string) error {
	h.Init(ctx, "", maxOpen, maxIdle, lifetime)
	migs, err := jobsMigrations()
	if err != nil {
		return err
	}

	return h.Migrate(ctx, migs)
}

func (s *Store) Insert(ctx context.Context, jb model.Job) (int64, error) {
	// log.Printf("inserting job in db: %+v", jb)
	db, err := s.db.GetConn()
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

	return jid, nil
}

func (s *Store) Remove(ctx context.Context, jid int) error {
	db, err := s.db.GetConn()
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

func (s *Store) RemoveMany(ctx context.Context, jids []int) error {
	db, err := s.db.GetConn()
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

func (s *Store) ByID(ctx context.Context, jid int) (model.Job, error) {
	var job model.Job
	db, err := s.db.GetConn()
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

func (s *Store) ByUID(ctx context.Context, uid int) ([]model.Job, error) {
	var jobs []model.Job
	db, err := s.db.GetConn()
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
		var job model.Job

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

func (s *Store) ByUIDs(ctx context.Context, uids []int) ([]model.Job, error) {
	var jobs []model.Job

	if len(uids) == 0 {
		return jobs, nil // no users, empty list
	}

	db, err := s.db.GetConn()
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
		var job model.Job

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

func (s *Store) All(ctx context.Context, limit, offset string) ([]model.Job, error) {
	var jobs []model.Job
	db, err := s.db.GetConn()
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
		var job model.Job
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

func (s *Store) Update(ctx context.Context, jb model.Job) error {
	db, err := s.db.GetConn()
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

func (s *Store) MarkStatus(ctx context.Context, jid int64, status string, duration time.Duration) error {
	db, err := s.db.GetConn()
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

// SaveLog stores (replaces) the kept output of a job.
func (s *Store) SaveLog(ctx context.Context, jid int64, text string) error {
	db, err := s.db.GetConn()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO job_logs (jid, log, updatedAt) VALUES (?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET log = excluded.log, updatedAt = excluded.updatedAt`,
		jid, text, ut.CurrentTime())

	return err
}

// Log returns the kept output of a job ("" if none was saved).
func (s *Store) Log(ctx context.Context, jid int64) (string, error) {
	db, err := s.db.GetConn()
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

// CountByEngine counts the jobs per recorded engine ("" = none recorded).
func (s *Store) CountByEngine(ctx context.Context) (map[string]int, error) {
	db, err := s.db.GetConn()
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

// SetReason records why a job is in its state.
func (s *Store) SetReason(ctx context.Context, jid int64, reason string) error {
	db, err := s.db.GetConn()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE jobs SET status_reason = ? WHERE jid = ?`, reason, jid)

	return err
}

// MarkEngine records where a job runs.
func (s *Store) MarkEngine(ctx context.Context, jid int64, engine string) error {
	db, err := s.db.GetConn()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE jobs SET engine = ? WHERE jid = ?`, engine, jid)

	return err
}
