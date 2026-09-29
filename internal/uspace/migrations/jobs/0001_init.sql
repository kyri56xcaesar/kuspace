-- jobs.db as it was before migrations (baseline: a no-op on existing
-- databases). engine and gid came later: 0002.
CREATE TABLE IF NOT EXISTS jobs (
    jid INTEGER PRIMARY KEY AUTOINCREMENT,
    uid INTEGER,
    description TEXT,
    duration REAL,
    input TEXT,
    inputFormat TEXT,
    output TEXT,
    outputFormat TEXT,
    logic TEXT,
    logicBody TEXT,
    logicHeaders TEXT,
    parameters TEXT,
    status TEXT,
    completed INTEGER,
    completedAt DATETIME,
    createdAt DATETIME,
    parallelism INTEGER,
    priority INTEGER,
    memoryRequest TEXT,
    cpuRequest TEXT,
    memoryLimit TEXT,
    cpuLimit TEXT,
    ephemeralStorageRequest TEXT,
    ephemeralStorageLimit TEXT
);

-- the kept tail of each job's output (outlives the live stream)
CREATE TABLE IF NOT EXISTS job_logs (
    jid INTEGER PRIMARY KEY,
    log TEXT,
    updatedAt DATETIME
);

CREATE TABLE IF NOT EXISTS apps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE,
    image TEXT,
    description TEXT,
    version TEXT,
    author TEXT,
    authorId INTEGER,
    status TEXT,
    insertedAt DATETIME,
    createdAt DATETIME
);
