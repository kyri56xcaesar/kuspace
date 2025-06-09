package minioth

/*
* A minioth handler, encircling a DuckDB.
*
* */

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"

	// duckdb driver
	_ "github.com/marcboeker/go-duckdb"
)

/* init sql script */
const (
	initSQL string = `
  CREATE TABLE IF NOT EXISTS users (
		uid INTEGER,
		username TEXT,
		info TEXT,
		home TEXT,
		shell TEXT,
		pgroup INTEGER
	);
	CREATE TABLE IF NOT EXISTS passwords (
		uid INTEGER,
		hashpass TEXT,
		lastPasswordChange TEXT,
		minimumPasswordAge TEXT,
		maximumPasswordAge TEXT,
		warningPeriod TEXT,
		inactivityPeriod TEXT,
		expirationDate TEXT
	);
	CREATE TABLE IF NOT EXISTS groups (
		gid INTEGER,
		groupname TEXT 
	);
  CREATE TABLE IF NOT EXISTS user_groups (
    uid INTEGER NOT NULL,
    gid INTEGER NOT NULL
  );
  `
)

// DBHandler struct containing reference to a database driver
// the Minioth cetrnalle
// and  a path
type DBHandler struct {
	db      *sql.DB
	minioth *Minioth
	DBpath  string
}

/* "singleton" like db connection reference */
func (m *DBHandler) getConn() (*sql.DB, error) {
	db := m.db
	var err error

	if db == nil {
		db, err = sql.Open(m.minioth.Config.MiniothDBDriver, m.DBpath)
		m.db = db
		if err != nil {
			log.Printf("Failed to connect to DuckDB: %v", err)

			return nil, err
		}
	}

	return db, err
}

/* initialization method for root user, could be reconfigured*/
func (m *DBHandler) insertRootUser(user ut.User, db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	userQuery := `
    INSERT INTO 
        users (uid, username, info, home, shell, pgroup) 
    VALUES 
        (?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(userQuery, user.UID, user.Username, user.Info, user.Home, user.Shell, user.Pgroup)
	if err != nil {
		err = tx.Rollback()
		if err != nil {
			return err
		}

		return fmt.Errorf("failed to insert root user: %w", err)
	}

	hashPass, err := hash([]byte(user.Password.Hashpass))
	if err != nil {
		log.Printf("failed to hash the pass: %v", err)
		err = tx.Rollback()
		if err != nil {
			return err
		}

		return fmt.Errorf("failed to insert rootUser, hashing failed: %w", err)
	}

	passwordQuery := `
    INSERT INTO 
        passwords (uid, hashpass, lastPasswordChange, minimumPasswordAge,
		 maximumPasswordAge, warningPeriod, inactivityPeriod, expirationDate) 
    VALUES 
        (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(passwordQuery, user.UID, hashPass, user.Password.LastPasswordChange, user.Password.MinimumPasswordAge,
		user.Password.MaximumPasswordAge, user.Password.WarningPeriod, user.Password.InactivityPeriod, user.Password.ExpirationDate)
	if err != nil {
		err = tx.Rollback()
		if err != nil {
			return err
		}

		return fmt.Errorf("failed to insert root password: %w", err)
	}

	usergroupQuery := `
    INSERT INTO
      user_groups (uid, gid)
    VALUES
      (?, ?)`
	_, err = tx.Exec(usergroupQuery, user.UID, 0)
	if err != nil {
		err = tx.Rollback()
		if err != nil {
			return err
		}

		return fmt.Errorf("failed to group root user: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Init method of Database Minioth Handler
// performs all needed initialization calls and commands
func (m *DBHandler) Init() {
	log.Print("[INIT_DB]Initializing... Minioth DB")
	_, err := os.Stat("data")
	if err != nil {
		err = os.Mkdir("data", 0o700)
		if err != nil {
			panic("failed to make new directory.")
		}
	}

	cpath, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parts := strings.Split(m.DBpath, "/")
	if strings.HasSuffix(m.DBpath, "/") || len(parts) == 0 {
		panic("invalid db path value")
	}
	dbPath := strings.Join(parts[:len(parts)-1], "/")
	err = os.MkdirAll(cpath+"/"+dbPath, 0o644)
	if err != nil {
		panic(err)
	}

	db, err := m.getConn()
	if err != nil {
		panic("destructive")
	}

	// set ref to db
	m.db = db

	_, err = db.Exec(initSQL)
	if err != nil {
		log.Fatal("Failed to create tables:", err)
	}

	// Check for main group existence
	log.Print("[INIT_DB]Checking for main groups...")
	var mainGroupsExist bool
	err = db.QueryRow("SELECT EXISTS(SELECT 2 FROM groups WHERE gid = 0 OR gid = 1000)").Scan(&mainGroupsExist)
	if err != nil {
		log.Fatalf("Failed to query groups")
	}

	if !mainGroupsExist {
		log.Print("[INIT_DB]Inserting main groups: admin/user -> gid: 0/1000")

		query := `
      INSERT INTO
        groups (gid, groupname)
      VALUES 
        (0, 'admin'),
        (100, 'mod'),
        (1000, 'user');`

		_, err = db.Exec(query, nil)
		if err != nil {
			log.Fatalf("failed to insert groups: %v", err)
		}
	} else {
		log.Print("[INIT_DB]groups already exist!")
	}

	log.Print("[INIT_DB]Checking for root user...")
	// Check if the root user already exists
	var rootExists bool
	err = db.QueryRow(fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM users WHERE username = '%s')", m.minioth.Config.MiniothAccessKey)).Scan(&rootExists)
	if err != nil {
		log.Fatalf("Failed to check for root user: %v", err)
	}

	if !rootExists {
		log.Print("[INIT_DB]Inserting root user...")
		// Directly insert the root user with UID 0

		err = m.insertRootUser(ut.User{
			Username: m.minioth.Config.MiniothAccessKey,
			UID:      0,
			Pgroup:   0,
			Password: ut.Password{
				Hashpass: m.minioth.Config.MiniothSecretKey,
			},
		}, db)
		if err != nil {
			log.Fatalf("Failed to insert root user: %v", err)
		}
	} else {
		log.Print("[INIT_DB]Root user already exists")
	}
}

// Useradd method
/* will insert a given user in the relational db
*
* relations:
* passwords, user_groups, groups
*
* Each user should be associated with his own group
* */
func (m *DBHandler) Useradd(user ut.User) (int, int, error) {
	db, err := m.getConn()
	if err != nil {
		return -1, -1, err
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)

		return -1, -1, err
	}

	// check if user exists...
	var exists int
	err = db.QueryRow("SELECT 1 FROM users WHERE username = ?", user.Username).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("User with name %q does not exist.", user.Username)
	} else if err != nil {
		log.Printf("Error checking for user existence: %v", err)

		return -1, -1, fmt.Errorf("error checking for user existence: %w", err)
	} else {
		log.Printf("User with name %q already exists.", user.Username)

		return -1, -1, errors.New("user already exists")
	}

	userQuery := `
  INSERT INTO 
    users (uid, username, info, home, shell, pgroup) 
  VALUES 
    (?, ?, ?, ?, ?, ?)
  `

	user.UID, err = m.nextID("users")
	if err != nil {
		log.Printf("failed to retrieve the next avaible uid: %v", err)

		return -1, -1, err
	}
	user.Pgroup = user.UID

	_, err = tx.Exec(userQuery, user.UID, user.Username, user.Info, user.Home, user.Shell, user.Pgroup)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
		err = tx.Rollback()
		if err != nil {
			return -1, -1, err
		}

		return -1, -1, fmt.Errorf("failed to add user: %w", err)
	}

	passwordQuery := `
  INSERT INTO
    passwords (uid, hashpass, lastpasswordchange, minimumpasswordage, maximumpasswordage, warningperiod, inactivityperiod, expirationdate)
  VALUES (?, ?, ?, ?, ?, ?, ?, ?)
  `
	/* add the user inique group */
	gid, err := m.Groupadd(ut.Group{Groupname: user.Username, GID: user.UID})
	if err != nil {
		log.Printf("failed to insert user unique/primary group: %v", err)

		return -1, -1, err
	}

	usergroupQuery := `
    INSERT INTO
      user_groups (uid, gid)
    VALUES
      (?, ?),
      (?, ?)`
	_, err = tx.Exec(usergroupQuery, user.UID, 1000, user.UID, gid)
	if err != nil {
		err = tx.Rollback()
		if err != nil {
			return -1, -1, err
		}
		log.Printf("failed to group user: %v", err)

		return -1, -1, fmt.Errorf("failed to execute insertion: %w", err)
	}

	hashPass, err := hash([]byte(user.Password.Hashpass))
	if err != nil {
		log.Printf("failed to hash the pass: %v", err)
		err = tx.Rollback()
		if err != nil {
			return -1, -1, err
		}

		return -1, -1, fmt.Errorf("failed to execute insertion: %w", err)
	}

	_, err = tx.Exec(passwordQuery, user.UID, hashPass, ut.CurrentTime(), user.Password.MinimumPasswordAge,
		user.Password.MaximumPasswordAge, user.Password.WarningPeriod, user.Password.InactivityPeriod, user.Password.ExpirationDate)
	if err != nil {
		err = tx.Rollback()
		if err != nil {
			return -1, -1, err
		}
		log.Printf("failed to execute query: %v", err)

		return -1, -1, fmt.Errorf("failed to execute insertion: %w", err)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("failed to commit transaction: %v", err)

		return -1, -1, err
	}

	return user.UID, gid, nil
}

// Userdel method of Database MiniotH Handler
// deletes the user with the given uid
func (m *DBHandler) Userdel(uid string) error {
	if err := checkIfRoot(uid); err != nil {
		log.Print("can't delete the root...")

		return fmt.Errorf("deleting the root?%v", nil)
	}

	db, err := m.getConn()
	if err != nil {
		log.Printf("failed to get db conn: %v", err)

		return err
	}

	deleteUserQuery := `DELETE FROM users WHERE uid = ?`
	deletePasswordQuery := `DELETE FROM passwords WHERE uid = ?`
	deleteUserGroupQuery := `DELETE FROM user_groups WHERE uid = ?`

	var (
		gid           int
		pgroupDeleted bool
	)
	err = db.QueryRow(`
    SELECT 
      gid 
    FROM 
      groups 
    WHERE groupname = (
      SELECT 
        username
      FROM 
        users 
      WHERE 
        uid = ?
    )`, uid).Scan(&gid)
	if err != nil {
		log.Printf("failed to retrieve primary group gid of the user")
		pgroupDeleted = true
	}

	if !pgroupDeleted {
		deletePrimaryGroupQuery := `
      DELETE FROM 
        groups 
      WHERE 
        gid = ?
      `
		cleanRemenantsQuery := `
      DELETE FROM 
        user_groups 
      WHERE 
        gid = ?
    `
		_, err = db.Exec(deletePrimaryGroupQuery, gid)
		if err != nil {
			log.Printf("error, failed to delete user primary group: %v", err)

			return err
		}

		_, err = db.Exec(cleanRemenantsQuery, gid)
		if err != nil {
			log.Printf("error, failed to clean the user_group to the deleted group relation: %v", err)

			return err
		}
	}

	_, err = db.Exec(deleteUserGroupQuery, uid)
	if err != nil {
		log.Printf("error, failed to delete usergroups: %v", err)

		return err
	}

	_, err = db.Exec(deletePasswordQuery, uid)
	if err != nil {
		log.Printf("error, failed to delete password: %v", err)

		return err
	}

	res, err := db.Exec(deleteUserQuery, uid)
	if err != nil {
		log.Printf("error, failed to delete user: %v", err)

		return err
	}

	rAffected, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to get the rows affected")

		return err
	}

	if rAffected == 0 {
		log.Print("no users were deleted")

		return errors.New("user not found")
	}

	return nil
}

// Usermod method of Database Minioth Handler
// replaces the existing user with the given UID with the given  user
func (m *DBHandler) Usermod(user ut.User) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("Failed to get DB connection: %v", err)

		return err
	}

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)

		return err
	}

	// Rollback in case of any error
	defer func() {
		if err != nil {
			log.Printf("Rolling back transaction due to error: %v", err)
			err = tx.Rollback()
			if err != nil {
				log.Printf("failed to rollback")
			}
		}
	}()

	// Step 1: Delete dependent records
	deleteUserGroupsQuery := `DELETE FROM user_groups WHERE uid = ?`
	_, err = tx.Exec(deleteUserGroupsQuery, user.UID)
	if err != nil {
		log.Printf("Failed to delete user-group associations: %v", err)

		return fmt.Errorf("failed to delete user-group associations: %w", err)
	}

	deletePasswordQuery := `DELETE FROM passwords WHERE uid = ?`
	_, err = tx.Exec(deletePasswordQuery, user.UID)
	if err != nil {
		log.Printf("Failed to delete password: %v", err)

		return fmt.Errorf("failed to delete password: %w", err)
	}

	// Step 2: Update the `users` table
	updateUserQuery := `
    UPDATE 
      users 
    SET 
      username = ?, info = ?, home = ?, shell = ? 
    WHERE 
      uid = ?;
  `
	res, err := tx.Exec(updateUserQuery, user.Username, user.Info, user.Home, user.Shell, user.UID)
	if err != nil {
		log.Printf("failed to update user: %v", err)

		return fmt.Errorf("failed to update user: %w", err)
	}

	if rAff, err := res.RowsAffected(); err != nil {
		log.Printf("failed to get rows affected: %v", err)

		return err
	} else if rAff == 0 {
		return ut.NewWarning("user with uid: %v doesn't exist", user.UID)
	}

	hashPass, err := hash([]byte(user.Password.Hashpass))
	if err != nil {
		log.Printf("failed to hash the pass: %v", err)

		return err
	}

	// Step 3: Reinsert into `passwords`
	insertPasswordQuery := `
    INSERT INTO 
      passwords (uid, hashpass, lastpasswordchange, minimumpasswordage, maximumpasswordage, warningperiod, inactivityperiod, expirationdate)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?);
  `
	_, err = tx.Exec(insertPasswordQuery, user.UID, hashPass, ut.CurrentTime(),
		user.Password.MinimumPasswordAge, user.Password.MaximumPasswordAge, user.Password.WarningPeriod,
		user.Password.InactivityPeriod, user.Password.ExpirationDate)
	if err != nil {
		log.Printf("Failed to insert password: %v", err)

		return fmt.Errorf("failed to insert password: %w", err)
	}

	// Step 4: Reinsert into `user_groups`
	if len(user.Groups) > 0 {
		insertUserGroupsQuery := `
      INSERT INTO 
        user_groups (uid, gid) 
      VALUES 
    `
		var params []any
		for i, group := range user.Groups {
			insertUserGroupsQuery += "(?, ?)"
			if i < len(user.Groups)-1 {
				insertUserGroupsQuery += ", "
			}
			params = append(params, user.UID, group.GID)
		}

		_, err = tx.Exec(insertUserGroupsQuery, params...)
		if err != nil {
			log.Printf("Failed to insert user-group associations: %v", err)

			return fmt.Errorf("failed to insert user-group associations: %w", err)
		}
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		log.Printf("Failed to commit transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Userpatch method of Database Minioth Handler
// changes the user by given UID, specific fields
func (m *DBHandler) Userpatch(uid string, fields map[string]any) error {
	query := "UPDATE users SET "
	args := []any{}

	var groups any
	var password string
	for key, value := range fields {
		switch key {
		case "uid":

			continue
		case "groups":
			groups = fields[key]

			continue
		case "password":
			password = fields[key].(string)
		default:
			if fields[key] == "" {
				continue
			}
			query += key + " = ?, "
			args = append(args, value)
		}
	}
	db, err := m.getConn()
	if err != nil {
		return fmt.Errorf("failed to connect to db: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if len(args) != 0 {
		// Key patches
		query = strings.TrimSuffix(query, ", ") + " WHERE uid = ?"
		args = append(args, uid)

		res, err := tx.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("failed to execute update query: %w", err)
		}
		if rAff, err := res.RowsAffected(); err != nil {
			return fmt.Errorf("checking rows affected: %w", err)
		} else if rAff == 0 {
			return ut.NewWarning("user with uid: %v doesn't exist", uid)
		}
	}

	// group patch
	// if groups arg is here, we need to update the relation
	if _, ok := groups.(string); ok && len(groups.(string)) > 0 {
		_, err := tx.Exec("DELETE FROM user_groups WHERE uid = ?", uid)
		if err != nil {
			log.Printf("failed to delete old relations..:%v", err)

			return fmt.Errorf("failed to delete old relations: %w", err)
		}

		groups := strings.Split(groups.(string), ",")       // Assuming `groups` is a string of comma-separated group names
		placeholders := strings.Repeat(",?", len(groups)-1) // Create placeholders for additional groups

		insQuery := `
      INSERT INTO 
          user_groups (uid, gid)
      SELECT 
          ?, gid
      FROM 
          groups
      WHERE 
          groupname IN (?` + placeholders + `)
    `
		args := []any{uid}
		for _, group := range groups {
			args = append(args, strings.TrimSpace(group))
		}

		_, err = tx.Exec(insQuery, args...)
		if err != nil {
			log.Printf("failed to insert user groups: %v", err)

			return fmt.Errorf("failed to insert user groups: %w", err)
		}
	}

	// password patch
	if password != "" {
		hashPass, err := hash([]byte(password))
		if err != nil {
			log.Printf("failed to hash the pass: %v", err)

			return err
		}
		pquery := `
      UPDATE 
        passwords 
      SET 
        hashpass = ?, lastpasswordchange = ?
      WHERE 
        uid = ?`

		_, err = tx.Exec(pquery, string(hashPass), ut.CurrentTime(), uid)
		if err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed to commit transaction: %v", err)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Groupadd method of Database Minioth Handler
// inserts a new Group in the database
// retrieves a new id
func (m *DBHandler) Groupadd(group ut.Group) (int, error) {
	db, err := m.getConn()
	if err != nil {
		log.Printf("failed to get the db conn: %v", err)

		return -1, err
	}

	// check if group exists...
	var exists int
	err = db.QueryRow("SELECT 1 FROM groups WHERE groupname = ?", group.Groupname).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("group with name %q does not exist.", group.Groupname)
	} else if err != nil {
		log.Printf("errror checking for group existence: %v", err)

		return -1, fmt.Errorf("error checking for group existence: %w", err)
	} else {
		log.Printf("group with name %q already exists.", group.Groupname)

		return -1, errors.New("group already exists")
	}

	groupAddQuery := `
    INSERT INTO
      groups (gid, groupname)
    VALUES
      (?, ?);
    
  `

	// insert group
	gid, err := m.nextID("groups")
	if err != nil {
		log.Printf("failed to retrieve the nextid")

		return -1, err
	}

	_, err = db.Exec(groupAddQuery, gid, group.Groupname)
	if err != nil {
		log.Printf("error executing groupAddQuery: %v", err)

		return -1, err
	}

	// "update" or insert group user relation
	if len(group.Users) > 0 {
		placeholders := strings.Repeat("(?, ?),", len(group.Users))
		placeholders = strings.TrimSuffix(placeholders, ",") // Remove trailing comma
		userGroupQuery := "INSERT INTO user_groups (uid, gid) VALUES " + placeholders

		args := []any{}
		for _, user := range group.Users {
			args = append(args, user.UID, gid)
		}

		_, err = db.Exec(userGroupQuery, args...)
		if err != nil {
			log.Printf("Error executing userGroupQuery: %v", err)

			return -1, err
		}
	}

	return gid, nil
}

// Groupdel method of Database Minioth Handler
// deletes a group given a gid
func (m *DBHandler) Groupdel(gid string) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("failed to get db conn: %v", err)

		return err
	}
	groupDelQuery := `DELETE FROM groups WHERE gid = ?`
	userGroupDel := `DELETE FROM user_groups WHERE gid = ?`

	res, err := db.Exec(groupDelQuery, gid)
	if err != nil {
		log.Printf("error, failed to delete group: %v", err)

		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		log.Printf("error getting rows affected num: %v", err)

		return err
	}

	if rowsAffected == 0 {
		log.Printf("group: %q doesn't exist", gid)

		return errors.New("group doens't exist")
	}

	_, err = db.Exec(userGroupDel, gid)
	if err != nil {
		log.Printf("error, failed to delete usergroups: %v", err)

		return err
	}

	return nil
}

// Groupmod method of Database Minioth Handler
// replaces the given group (with gid) completely
func (m *DBHandler) Groupmod(group ut.Group) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("Failed to get DB connection: %v", err)

		return err
	}

	// Update group information
	updateGroupQuery := `
    UPDATE groups
    SET groupname = ?
    WHERE gid = ?;
  `
	res, err := db.Exec(updateGroupQuery, group.Groupname, group.GID)
	if err != nil {
		log.Printf("Failed to update group: %v", err)

		return err
	}

	if rAff, err := res.RowsAffected(); err != nil {
		log.Printf("failed to get rows affected by query: %v", err)

		return err
	} else if rAff == 0 {
		return ut.NewWarning("group with gid %v doesn't exist", group.GID)
	}

	// Update user-group relations
	deleteUserGroupsQuery := `DELETE FROM user_groups WHERE gid = ?`
	_, err = db.Exec(deleteUserGroupsQuery, group.GID)
	if err != nil {
		log.Printf("Failed to delete user-group associations: %v", err)

		return err
	}

	if len(group.Users) > 0 {
		placeholders := strings.Repeat("(?, ?),", len(group.Users))
		placeholders = strings.TrimSuffix(placeholders, ",")
		insertUserGroupsQuery := "INSERT INTO user_groups (uid, gid) VALUES " + placeholders

		args := []any{}
		for _, user := range group.Users {
			args = append(args, user.UID, group.GID)
		}

		_, err = db.Exec(insertUserGroupsQuery, args...)
		if err != nil {
			log.Printf("Failed to insert user-group associations: %v", err)

			return err
		}
	}

	return nil
}

// Grouppatch method of Database Minioth Handler
// changes the gid given group given fields
func (m *DBHandler) Grouppatch(gid string, fields map[string]any) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("Failed to get DB connection: %v", err)

		return err
	}

	// Build the dynamic update query
	query := "UPDATE groups SET "
	args := []any{}
	for field, value := range fields {
		query += field + " = ?, "
		args = append(args, value)
	}
	query = strings.TrimSuffix(query, ", ") // Remove the trailing comma
	query += " WHERE gid = ?"
	args = append(args, gid)

	res, err := db.Exec(query, args...)
	if err != nil {
		log.Printf("failed to patch group: %v", err)

		return err
	}

	rAff, err := res.RowsAffected()
	if err != nil {
		log.Printf("failed to get rows affected: %v", err)

		return err
	}

	if rAff == 0 {
		return ut.NewWarning("group with uid: %v doesn't exist", gid)
	}

	return nil
}

// Passwd method of Database Minioth Handler
// responsible to change the password of the given user
// arguments: user name - new password
//
// ! warning ! this method changes password no questions asked!
func (m *DBHandler) Passwd(username, password string) error {
	db, err := m.getConn()
	if err != nil {
		log.Printf("failed to connect to database: %v", err)

		return err
	}

	hashPass, err := hash([]byte(password))
	if err != nil {
		log.Printf("failed to hash the pass: %v", err)

		return err
	}

	updateQuery := `
    UPDATE 
      passwords  
    SET 
      hashpass = ?,
      lastPasswordChange = ? 
    WHERE 
      uid = (
        SELECT 
          uid 
        FROM 
          users  
        WHERE 
          username = ?
      );
  `
	_, err = db.Exec(updateQuery, hashPass, ut.CurrentTime(), username)
	if err != nil {
		log.Printf("failed to exec update query: %v", err)

		return err
	}

	return nil
}

// Select method of Database Minioth Handler
// returns the chosen field
// @argument: id - users or groups
// returns all
func (m *DBHandler) Select(id string) any {
	var param, value string
	parts := strings.Split(id, "?")
	if len(parts) > 1 {
		id = parts[0]
		parts2 := strings.Split(parts[1], "=")
		if len(parts2) > 1 {
			param = parts2[0]
			value = parts2[1]
		}
	}
	db, err := m.getConn()
	if err != nil {
		log.Printf("failed to connect to database: %v", err)

		return nil
	}

	switch id {
	case "users":
		var (
			result    []any
			userQuery string
			rows      *sql.Rows
			err       error
		)

		if param != "" && value != "" {
			userQuery = fmt.Sprintf(`
			SELECT  
			  u.uid, u.username, p.hashpass, p.lastPasswordChange, p.minimumPasswordAge,
			  p.maximumPasswordAge, p.warningPeriod, p.inactivityPeriod, p.expirationDate,
			  u.info, u.home, u.shell, u.pgroup, GROUP_CONCAT(g.groupname), GROUP_CONCAT(g.gid) as groups
			FROM 
			  users u
			LEFT JOIN passwords p ON p.uid = u.uid
			LEFT JOIN user_groups ug ON ug.uid = u.uid
			LEFT JOIN groups g ON g.gid = ug.gid
			WHERE 
			  u.%s = ?
			GROUP BY 
			  u.uid, u.username, u.info, u.home, u.shell, u.pgroup, p.hashpass, p.lastPasswordChange, p.minimumPasswordAge, p.maximumPasswordAge, p.warningPeriod, p.inactivityPeriod, p.expirationDate;
	  		`, param)

			rows, err = db.Query(userQuery, value)
		} else {
			userQuery = `
			SELECT  
			  u.uid, u.username, p.hashpass, p.lastPasswordChange, p.minimumPasswordAge,
			  p.maximumPasswordAge, p.warningPeriod, p.inactivityPeriod, p.expirationDate,
			  u.info, u.home, u.shell, u.pgroup, GROUP_CONCAT(g.groupname), GROUP_CONCAT(g.gid) as groups
			FROM 
			  users u
			LEFT JOIN passwords p ON p.uid = u.uid
			LEFT JOIN user_groups ug ON ug.uid = u.uid
			LEFT JOIN groups g ON g.gid = ug.gid
			GROUP BY 
			  u.uid, u.username, u.info, u.home, u.shell, u.pgroup, p.hashpass, p.lastPasswordChange, p.minimumPasswordAge, p.maximumPasswordAge, p.warningPeriod, p.inactivityPeriod, p.expirationDate;
	  		`
			rows, err = db.Query(userQuery)
		}
		if err != nil {
			log.Printf("failed to query users: %v", err)

			return nil
		}
		defer func() {
			err := rows.Close()
			if err != nil {
				log.Printf("failed to close rows: %v", err)
			}
		}()

		for rows.Next() {
			var user ut.User
			var groupNames sql.NullString // Use sql.NullString to handle NULL values
			var grouIDs sql.NullString
			err := rows.Scan(&user.UID, &user.Username, &user.Password.Hashpass, &user.Password.LastPasswordChange, &user.Password.MinimumPasswordAge, &user.Password.MaximumPasswordAge, &user.Password.WarningPeriod, &user.Password.InactivityPeriod, &user.Password.ExpirationDate, &user.Info, &user.Home, &user.Shell, &user.Pgroup, &groupNames, &grouIDs)
			if err != nil {
				log.Printf("failed to scan user: %v", err)

				return nil
			}

			groups := []ut.Group{}
			if groupNames.Valid && groupNames.String != "" && grouIDs.Valid && grouIDs.String != "" { // Check if groupNames is valid and not empty
				groupNameList := strings.Split(groupNames.String, ",")
				groupIDsList := strings.Split(grouIDs.String, ",")
				for i, groupName := range groupNameList {
					gid, err := strconv.Atoi(groupIDsList[i])
					if err != nil {
						log.Printf("failed to atoi gid from: %v", groupIDsList[i])
					}
					groups = append(groups, ut.Group{
						Groupname: groupName,
						GID:       gid,
					})
				}
			}
			user.Groups = groups

			result = append(result, user)
		}

		return result
	case "groups":
		var result []any

		groupQuery := `
      SELECT 
        g.gid, g.groupname, GROUP_CONCAT(u.username), GROUP_CONCAT(u.uid) as users
      FROM 
        groups g
      LEFT JOIN user_groups ug ON g.gid = ug.gid
      LEFT JOIN users u ON u.uid = ug.uid
      GROUP BY 
        g.gid, g.groupname;
    `
		rows, err := db.Query(groupQuery)
		if err != nil {
			log.Printf("failed to query groups: %v", err)

			return nil
		}
		defer func() {
			err := rows.Close()
			if err != nil {
				log.Printf("failed to close rows: %v", err)
			}
		}()

		for rows.Next() {
			var group ut.Group
			var userNames sql.NullString
			var userIDs sql.NullString
			err := rows.Scan(&group.GID, &group.Groupname, &userNames, &userIDs)
			if err != nil {
				log.Printf("failed to scan group: %v", err)

				return nil
			}

			// Parse user names into dummy User structs

			users := []ut.User{}
			if userNames.Valid && userNames.String != "" {
				userNameList := strings.Split(userNames.String, ",")
				userIDsList := strings.Split(userIDs.String, ",")
				for i, userName := range userNameList {
					uid, err := strconv.Atoi(userIDsList[i])
					if err != nil {
						log.Printf("failed to atoi a uid: %v", userIDsList[i])

						return nil
					}
					users = append(users, ut.User{
						Username: userName,
						UID:      uid,
					})
				}
			}
			group.Users = users
			// Append group as an any
			result = append(result, group)
		}

		return result
	default:
		if verbose {
			log.Printf("Invalid id: %s", id)
		}

		return nil
	}
}

// Authenticate method of Database Minioth Handler
// checks for proof of existence for a given pair of user creds
func (m *DBHandler) Authenticate(username, password string) (ut.User, error) {
	db, err := m.getConn()
	if err != nil {
		return ut.User{}, err
	}
	user := getUser(username, db)
	if user.Username == "" {
		return ut.User{}, errors.New("user not found")
	}

	if verifyPass([]byte(user.Password.Hashpass), []byte(password)) {
		return user, nil
	}

	return ut.User{}, fmt.Errorf("failed to authenticate bad credentials: %v", nil)
}

// Purge method of Database Minioth Handler
// not working yet
// will delete everything ...
func (m *DBHandler) Purge() {
	// todo
}

// Close method of Database Minioth Handler
// forwards to the close database call
// @look utils/database
// close the prev "singleton" db connection */
func (m *DBHandler) Close() {
	if m.db != nil {
		err := m.db.Close()
		if err != nil {
			log.Printf("failed to close database connection: %v", err)
		}
	}
}

/* somewhat UTILITY functions and methods */
/* select all user information given a username */
func getUser(username string, db *sql.DB) ut.User {
	// lets check if the user exists before joining the big guns
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)", username).Scan(&exists)
	if err != nil {
		log.Printf("failed to check if user exists: %v", err)
	}

	if !exists {
		return ut.User{}
	}

	userQuery := `
    SELECT 
      u.username, u.info, u.home, u.shell, u.uid, u.pgroup,
      g.gid, g.groupname
    FROM 
      users u
    LEFT JOIN
      user_groups ug ON u.uid = ug.uid
    LEFT JOIN
      groups g ON ug.gid = g.gid
    WHERE 
      username = ?
    `

	rows, err := db.Query(userQuery, username)
	if err != nil {
		log.Printf("error on query: %v", err)

		return ut.User{}
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	user := ut.User{}
	groups := make([]ut.Group, 0)

	var (
		gid   sql.NullInt64
		gname sql.NullString
	)

	for rows.Next() {
		if err := rows.Scan(&user.Username, &user.Info, &user.Home, &user.Shell, &user.UID, &user.Pgroup, &gid, &gname); err != nil {
			log.Printf("failed to ugr scan row: %v", err)

			return ut.User{}
		}

		if gid.Valid && gname.Valid {
			groups = append(groups, ut.Group{
				GID:       int(gid.Int64),
				Groupname: gname.String,
			})
		}
	}

	user.Groups = groups

	passwordQuery := `
    SELECT 
      hashpass, lastPasswordChange, minimumPasswordAge, maximumPasswordAge,
      warningPeriod, inactivityPeriod, expirationDate
    FROM 
      passwords 
    WHERE 
      uid = ?`
	password := ut.Password{}
	row := db.QueryRow(passwordQuery, user.UID)
	if row == nil {
		return ut.User{}
	}

	err = row.Scan(&password.Hashpass, &password.LastPasswordChange, &password.MinimumPasswordAge,
		&password.MaximumPasswordAge, &password.WarningPeriod, &password.InactivityPeriod, &password.ExpirationDate)
	if err != nil {
		log.Printf("failed to scan password: %v", err)

		return ut.User{}
	}
	user.Password = password

	return user
}

func (m *DBHandler) nextID(table string) (int, error) {
	db, err := m.getConn()
	if err != nil {
		return 0, fmt.Errorf("failed to connect to database: %w", err)
	}

	var id, query string
	switch table {
	case "users":
		id = "uid"
		query = "SELECT COALESCE(MAX(uid), 999) + 1 FROM " + table + " WHERE " + id + " >= 1000"
	case "groups":
		id = "gid"
		query = "SELECT COALESCE(MAX(gid), 999) + 1 FROM " + table + " WHERE " + id + " >= 1000"
	}

	var nextUID int
	err = db.QueryRow(query).Scan(&nextUID)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve next UID: %w", err)
	}

	return nextUID, nil
}

func checkIfRoot(uid string) error {
	iuid, err := strconv.Atoi(uid)
	if err != nil {
		log.Printf("failed to atoi: %v", err)

		return err
	}

	if iuid == 0 {
		return fmt.Errorf("indeed root: %v", nil)
	}

	return nil
}
