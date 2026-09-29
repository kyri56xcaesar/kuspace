package uspace

/*
	The execution engines uspace can run jobs on, and which one takes new
	jobs. Both executors exist from the start; an admin switches between
	them at runtime (PUT /admin/executor), and the choice is kept in
	jobs.db's settings (it overrides J_EXECUTOR, the initial default).
	A job stays on the engine it started on: it records it (jobs.engine), and
	cancelling, following it again after a restart and settling it use that
	engine, not the current one.
*/

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"kyri56xcaesar/kuspace/internal/model"
	ut "kyri56xcaesar/kuspace/internal/utils"
)

// ErrEngineInactive refuses a switch to an engine that doesn't answer.
var ErrEngineInactive = fmt.Errorf("the engine is not active (%w)", ut.ErrUnavailable)

// engineSet holds the executors by name and the one taking new jobs.
type engineSet struct {
	mu      sync.RWMutex
	byName  map[string]JobExecutor
	order   []string // as added: the order the admin page lists them
	current string
}

func newEngineSet(executors ...JobExecutor) *engineSet {
	e := &engineSet{byName: map[string]JobExecutor{}}
	for _, x := range executors {
		e.byName[x.Name()] = x
		e.order = append(e.order, x.Name())
	}
	if len(e.order) > 0 {
		e.current = e.order[0]
	}

	return e
}

// engineName maps J_EXECUTOR's values (and aliases) to an engine name.
func engineName(s string) string {
	switch s {
	case "local", "default", "docker":
		return "docker"
	case "kubernetes", "k8s":
		return "kubernetes"
	default:
		return s
	}
}

// now returns the engine taking new jobs.
func (e *engineSet) now() (string, JobExecutor) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.current, e.byName[e.current]
}

// forJob returns the engine a job runs on (the current one for jobs that
// never recorded one).
func (e *engineSet) forJob(engine string) JobExecutor {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if x, ok := e.byName[engine]; ok {
		return x
	}

	return e.byName[e.current]
}

// get returns the engine called name.
func (e *engineSet) get(name string) (JobExecutor, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	x, ok := e.byName[name]

	return x, ok
}

// use makes name the engine for new jobs.
func (e *engineSet) use(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.byName[name]; !ok {
		return fmt.Errorf("no engine %q (%w)", name, ut.ErrNotFound)
	}
	e.current = name

	return nil
}

// statuses checks every engine (in parallel), the current one marked.
func (e *engineSet) statuses(ctx context.Context) []model.ExecutorStatus {
	e.mu.RLock()
	names, current := append([]string{}, e.order...), e.current
	e.mu.RUnlock()
	out := make([]model.ExecutorStatus, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = e.forJob(name).Status(ctx)
			out[i].Engine = name
			out[i].Current = name == current
		}()
	}
	wg.Wait()

	return out
}

// engineSetting is the stored choice of engine.
const engineSetting = "executor"

// storedEngine reads the engine an admin chose (empty when none).
func (srv *UService) storedEngine(ctx context.Context) (model.Setting, error) {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		return model.Setting{}, err
	}
	var s model.Setting
	err = db.QueryRowContext(ctx, `SELECT key, value, updated_by, updated_at FROM settings WHERE key = ?`, engineSetting).
		Scan(&s.Key, &s.Value, &s.UpdatedBy, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Setting{}, nil
	}

	return s, err
}

func (srv *UService) storeEngine(ctx context.Context, name, by string) error {
	db, err := srv.jdbh.GetConn()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO settings (key, value, updated_by, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_by = excluded.updated_by, updated_at = excluded.updated_at`,
		engineSetting, name, by, time.Now().UTC().Format(time.RFC3339))

	return err
}

// switchEngine makes name the engine for new jobs, if it is active, and
// keeps the choice. by is who asked (for the record).
func (srv *UService) switchEngine(ctx context.Context, engines *engineSet, name, by string) error {
	x, ok := engines.get(name)
	if !ok {
		return fmt.Errorf("no engine %q (%w)", name, ut.ErrNotFound)
	}
	if st := x.Status(ctx); !st.Active {
		return fmt.Errorf("%s: %s: %w", name, st.Error, ErrEngineInactive)
	}
	if err := srv.storeEngine(ctx, name, by); err != nil {
		return err
	}

	return engines.use(name)
}
