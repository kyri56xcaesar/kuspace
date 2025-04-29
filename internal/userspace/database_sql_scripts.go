package userspace

const (
	InitSql = `
    	CREATE TABLE IF NOT EXISTS resources (
    	  rid INTEGER PRIMARY KEY,
    	  uid INTEGER,
    	  gid INTEGER,
    	  vid INTEGER,
		  vname TEXT,
    	  size BIGINT,
    	  links INTEGER,
    	  perms TEXT,
    	  name TEXT,
		  path TEXT,
    	  type TEXT,
    	  created_at DATETIME,
    	  updated_at DATETIME,
    	  accessed_at DATETIME
    	);
    	CREATE TABLE IF NOT EXISTS volumes (
    	  vid INTEGER PRIMARY KEY,
    	  name TEXT,
    	  path TEXT,
		  dynamic BOOLEAN,
    	  capacity FLOAT,
    	  usage FLOAT,
		  created_at DATETIME
    	);
		CREATE TABLE IF NOT EXISTS userVolume(
			vid INTEGER,
			uid INTEGER,
			usage FLOAT,
			quota FLOAT,
			updated_at DATETIME
		);
  		CREATE TABLE IF NOT EXISTS groupVolume(
  		  vid INTEGER,
  		  gid INTEGER,
  		  usage FLOAT,
  		  quota FLOAT,
  		  updated_at DATETIME
  		);
    	CREATE SEQUENCE IF NOT EXISTS seq_resourceid START 1;
    	CREATE SEQUENCE IF NOT EXISTS seq_volumeid START 1; 
    `
	initSqlJobs = `
		CREATE TABLE IF NOT EXISTS jobs (
			jid INTEGER PRIMARY KEY,
			uid INTEGER,
			description TEXT,
			duration FLOAT,
			input TEXT,
			input_format TEXT,
			output TEXT,
			output_format TEXT,
			logic TEXT,
			logic_body TEXT,
			logic_headers TEXT,
			parameters TEXT,
			status TEXT,
			completed BOOLEAN,
			created_at DATETIME,
			completed_at DATETIME,
			parallelism INTEGER,
			priority INTEGER,
		);
		CREATE SEQUENCE IF NOT EXISTS seq_jobid START 1;
	`
)
