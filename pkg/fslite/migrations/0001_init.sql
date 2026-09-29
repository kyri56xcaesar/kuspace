-- fslite's database as it was before migrations (baseline: a no-op on
-- existing databases).
CREATE TABLE IF NOT EXISTS user_admin (
    uuid TEXT PRIMARY KEY,
    username TEXT UNIQUE,
    hashpass TEXT
);

CREATE TABLE IF NOT EXISTS volumes (
    vid INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE,
    path TEXT,
    dynamic BOOLEAN,
    capacity REAL,
    usage REAL,
    createdAt DATETIME
);

CREATE TABLE IF NOT EXISTS resources (
    rid INTEGER PRIMARY KEY AUTOINCREMENT,
    uid INTEGER,
    gid INTEGER,
    vid INTEGER,
    vname TEXT,
    size INTEGER,
    links INTEGER,
    perms TEXT,
    name TEXT,
    path TEXT,
    type TEXT,
    createdAt DATETIME,
    updatedAt DATETIME,
    accessedAt DATETIME,
    FOREIGN KEY (vid) REFERENCES volumes(vid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_volume (
    vid INTEGER,
    uid INTEGER,
    usage REAL,
    quota REAL,
    updatedAt DATETIME,
    PRIMARY KEY (vid, uid),
    FOREIGN KEY (vid) REFERENCES volumes(vid) ON DELETE CASCADE
);

-- faster lookup
CREATE INDEX IF NOT EXISTS idx_resources_vid ON resources(vid);
CREATE INDEX IF NOT EXISTS idx_resources_uid ON resources(uid);
CREATE INDEX IF NOT EXISTS idx_user_volume_uid ON user_volume(uid);

CREATE TABLE IF NOT EXISTS group_volume (
    vid INTEGER PRIMARY KEY,
    gid INTEGER NOT NULL,
    usage REAL NOT NULL DEFAULT 0,
    quota REAL NOT NULL DEFAULT 0,
    updatedAt DATETIME,
    FOREIGN KEY (vid) REFERENCES volumes(vid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_group_volume_gid ON group_volume(gid);

-- per-owner usage sums (see quota.go)
CREATE INDEX IF NOT EXISTS idx_resources_vid_uid ON resources(vid, uid);
