-- what the database dispatcher (J_DISPATCHER=database) needs to find its jobs
-- again after a restart; the in-memory dispatcher leaves them empty.
-- attempts: how often the job was claimed to run
ALTER TABLE jobs ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0;
-- until when (unix ms) a live watcher answers for the job; a claimed or
-- running job past it has lost its watcher
ALTER TABLE jobs ADD COLUMN lease_until INTEGER;
-- when (unix s) the engine started it
ALTER TABLE jobs ADD COLUMN started_unix INTEGER;
-- why the job is in its state, when that isn't obvious ("re-queued after a
-- restart", "lost: its container is gone", ...)
ALTER TABLE jobs ADD COLUMN status_reason TEXT;
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
