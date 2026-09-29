package uspace

import (
	"kyri56xcaesar/kuspace/internal/sqlite"
	"strings"
	"testing"
)

// jobs.db from every earlier shape ends up with the same columns, and its
// jobs stay readable.
func TestJobsDBMigratesEveryEarlierShape(t *testing.T) {
	baseline, err := jobsMigrationFiles.ReadFile("migrations/jobs/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	shapes := map[string]string{
		"new":                "",
		"before engine, gid": string(baseline),
		// the start-up code (ensureJobColumns) had added them
		"with ad-hoc columns": string(baseline) + `ALTER TABLE jobs ADD COLUMN engine TEXT; ALTER TABLE jobs ADD COLUMN gid INTEGER;`,
	}
	want := ""
	for name, schema := range shapes {
		t.Run(name, func(t *testing.T) {
			srv := &UService{}
			srv.jdbh = sqlite.NewDBHandler("jobs.db", t.TempDir()+"/", "sqlite3")
			srv.jdbh.Init(t.Context(), schema, "4", "2", "5")
			t.Cleanup(srv.jdbh.Close)
			db, _ := srv.jdbh.GetConn()
			if schema != "" {
				if _, err := db.ExecContext(t.Context(), `INSERT INTO jobs (jid, uid, description, duration, input, inputFormat,
					output, outputFormat, logic, logicBody, logicHeaders, parameters, status, completed, completedAt,
					createdAt, parallelism, priority, memoryRequest, cpuRequest, memoryLimit, cpuLimit,
					ephemeralStorageRequest, ephemeralStorageLimit)
				VALUES (5, 1001, '', 0, '/vol/in', '', '/vol/out', '', 'bash', '', '', '', 'completed', 1, '',
					'', 1, 0, '', '', '', '', '', '')`); err != nil {
					t.Fatal(err)
				}
			}
			if err := openJobsDB(t.Context(), &srv.jdbh, "4", "2", "5"); err != nil {
				t.Fatal(err)
			}
			rows, err := db.QueryContext(t.Context(), `SELECT name FROM pragma_table_info('jobs')`)
			if err != nil {
				t.Fatal(err)
			}
			var cols []string
			for rows.Next() {
				var c string
				_ = rows.Scan(&c)
				cols = append(cols, c)
			}
			_ = rows.Close()
			got := strings.Join(cols, ",")
			if !strings.Contains(got, ",engine,gid,") || !strings.HasSuffix(got, ",attempts,lease_until,started_unix,status_reason") {
				t.Errorf("columns: %s", got)
			}
			if want == "" {
				want = got
			} else if got != want {
				t.Errorf("columns differ between shapes:\n%s\n%s", got, want)
			}
			if schema != "" {
				job, err := srv.getJobByID(t.Context(), 5)
				if err != nil || job.UID != 1001 || job.Status != "completed" {
					t.Errorf("the job after migrating: %+v %v", job, err)
				}
			}
		})
	}
}
