-- settings an admin changes at runtime and uspace keeps across restarts
-- (the execution engine for new jobs: key "executor")
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_by TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);
