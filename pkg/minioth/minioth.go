package minioth

import (
	ut "kyri56xcaesar/kuspace/internal/utils"
	"log"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

/*
* WARNING: don't use the plain handler.
 */

/* hash cost for bcrypt hash function, reconfigurable from config*/
/*
*
* Variables */
var (
	auditLogPath   = "data/logs/minioth/audit.log"
	logPath        = "data/logs/minioth/minioth.log"
	forbiddenNames = []string{
		"root",
		"kubernetes",
		"k8s",
		"admin",
		"manager",
		"superuser",
		"sudo",
	}
	hashCostValue = 16
	verbose       = true
)

// Minioth is the core struct representing the user and group management system.
// It encapsulates the handler responsible for backend operations, the service layer,
// and the loaded configuration. Minioth provides a unified interface for managing
// users and groups, supporting operations such as creation, deletion, modification,
// authentication, and password management. The underlying handler can be configured
// to use different storage backends (e.g., plain file or database) as specified in
// the environment configuration. Minioth is designed to be extensible and integrates
// with custom utility types for configuration and user/group representations.
//
// Fields:
//   - handler: The MiniothHandler implementation responsible for backend operations.
//   - Service: The MService instance providing higher-level service logic.
//   - Config:  The loaded environment configuration (ut.EnvConfig) for Minioth.
type Minioth struct {
	handler MiniothHandler
	Service MService
	Config  ut.EnvConfig
}

// MiniothHandler defines an interface for managing users and groups, as well as authentication and lifecycle operations.
// It abstracts operations such as adding, deleting, modifying, and patching users and groups, as well as authentication and resource management.
//
// Methods:
//
//   - Init(): Initializes the handler and prepares any necessary resources.
//
//   - Useradd(user ut.User) (uid, pgroup int, err error):
//     Adds a new user. Returns the user's unique identifier (uid), the user's primary group ID (pgroup), and an error if the operation fails.
//
//   - Userdel(uid string) error:
//     Deletes a user identified by the given uid. Returns an error if the operation fails.
//
//   - Usermod(user ut.User) error:
//     Modifies an existing user with the provided user information. Returns an error if the operation fails.
//
//   - Userpatch(uid string, fields map[string]any) error:
//     Updates specific fields of a user identified by uid. The fields parameter is a map of field names to their new values. Returns an error if the operation fails.
//
//   - Groupadd(group ut.Group) (gid int, err error):
//     Adds a new group. Returns the group's unique identifier (gid) and an error if the operation fails.
//
//   - Groupdel(gid string) error:
//     Deletes a group identified by the given gid. Returns an error if the operation fails.
//
//   - Groupmod(group ut.Group) error:
//     Modifies an existing group with the provided group information. Returns an error if the operation fails.
//
//   - Grouppatch(gid string, fields map[string]any) error:
//     Updates specific fields of a group identified by gid. The fields parameter is a map of field names to their new values. Returns an error if the operation fails.
//
//   - Select(id string) any:
//     Retrieves a user or group by its identifier. Returns the corresponding object or nil if not found.
//
//   - Passwd(username, password string) error:
//     Changes the password for the specified username. Returns an error if the operation fails.
//
//   - Authenticate(username, password string) (ut.User, error):
//     Authenticates a user with the given username and password. Returns the authenticated user and an error if authentication fails.
//
//   - Purge():
//     Removes all users and groups, or resets the handler to its initial state.
//
//   - Close():
//     Releases any resources held by the handler and performs cleanup operations.
type MiniothHandler interface {
	Init()
	Useradd(user ut.User) (uid, pgroup int, err error) /* should return the uid as well*/
	Userdel(uid string) error
	Usermod(user ut.User) error
	Userpatch(uid string, fields map[string]any) error

	Groupadd(group ut.Group) (gid int, err error) /* should return the gid inserted as well*/
	Groupdel(gid string) error
	Groupmod(group ut.Group) error
	Grouppatch(gid string, fields map[string]any) error

	Select(id string) any
	Passwd(username, password string) error
	Authenticate(username, password string) (ut.User, error)

	Purge()
	Close()
}

// NewMinioth constructs and initializes a new Minioth instance using the configuration
// specified by the provided cfgPath. It loads environment configuration, sets up
// logging paths, hash cost, and verbosity, and selects the appropriate handler
// backend (such as database or plain file) based on configuration. The function
// also initializes the handler and service components of Minioth. If any configuration
// value is invalid (e.g., hash cost is not an integer), the function will log a fatal
// error and terminate the application.
//
// Parameters:
//   - cfgPath: The path to the configuration file to load.
//
// Returns:
//   - Minioth: A fully initialized Minioth instance ready for use.
//
// Package minioth provides a core user and group management system with pluggable backends.
// It supports operations such as adding, deleting, modifying, and authenticating users and groups.
// The package is configurable via environment variables and supports both plain file and database handlers.
// Passwords are securely hashed using bcrypt, with a configurable hash cost.
// Minioth is designed to be extensible and integrates with custom utility types and configuration loading.
//
// WARNING: The plain handler is not recommended for production use due to security concerns.
func NewMinioth(cfgPath string) Minioth {
	log.Print("[INIT]Creating new minioth...")
	newM := Minioth{
		Config: ut.LoadConfig(cfgPath),
	}

	var err error
	hashCostValue, err = strconv.Atoi(newM.Config.HashCost)
	if err != nil {
		log.Fatalf("failed to atoi hashCost var: %v", err)
	}

	auditLogPath = newM.Config.MiniothAuditLogs
	logPath = newM.Config.APILogsPath

	log.Printf("[INIT]setting log path to: %s", logPath)
	log.Printf("[INIT]setting audit log path to: %s", auditLogPath)
	log.Printf("[INIT]setting hashcost to: hashCost=%v", hashCostValue)

	newM.handler = handlerFactory(&newM)
	newM.Service = NewMSerivce(&newM)
	newM.handler.Init()

	verbose = newM.Config.Verbose
	return newM
}

// handlerFactory will producde the chosen "storage" system specified in the configuration
func handlerFactory(minioth *Minioth) MiniothHandler {
	switch minioth.Config.MiniothHandler {
	case "db", "database":
		return &DBHandler{DBpath: minioth.Config.MiniothDb, minioth: minioth}
	case "plain", "text", "file":
		return &PlainHandler{minioth: minioth}
	default:
		log.Fatal("not a valid handler, cannot operate")
		return nil
	}
}

// Useradd wrapper forwarding to Minioth handler
func (m *Minioth) Useradd(user ut.User) (int, int, error) {
	return m.handler.Useradd(user)
}

// Userdel wrapper forwarding to Minioth handler
func (m *Minioth) Userdel(username string) error {
	return m.handler.Userdel(username)
}

// Usermod wrapper forwarding to Minioth handler
func (m *Minioth) Usermod(user ut.User) error {
	return m.handler.Usermod(user)
}

// Userpatch wrapper forwarding to Minioth handler
func (m *Minioth) Userpatch(uid string, fields map[string]interface{}) error {
	return m.handler.Userpatch(uid, fields)
}

// Groupadd wrapper forwarding to Minioth handler
func (m *Minioth) Groupadd(group ut.Group) (int, error) {
	return m.handler.Groupadd(group)
}

// Groupdel wrapper forwarding to Minioth handler
func (m *Minioth) Groupdel(groupname string) error {
	return m.handler.Groupdel(groupname)
}

// Groupmod wrapper forwarding to Minioth handler
func (m *Minioth) Groupmod(group ut.Group) error {
	return m.handler.Groupmod(group)
}

// Grouppatch wrapper forwarding to Minioth handler
func (m *Minioth) Grouppatch(gid string, fields map[string]interface{}) error {
	return m.handler.Grouppatch(gid, fields)
}

// Passwd wrapper forwarding to Minioth handler
func (m *Minioth) Passwd(username, password string) error {
	return m.handler.Passwd(username, password)
}

// Select wrapper forwarding to minioth handler
func (m *Minioth) Select(id string) any {
	return m.handler.Select(id)
}

// Authenticate wrapper forwarding to minioth handler
func (m *Minioth) Authenticate(username, password string) (ut.User, error) {
	return m.handler.Authenticate(username, password)
}

// Sync is supposed to convert and check from one storage handler to another.
// WARNING. not implemented yet
/* NOTE: irrelevant atm
* This function should sync the DB state and the Plain state. TODO:*/
func (m *Minioth) Sync() error {
	return nil
}

/* UTIL functions */
/* use bcrypt blowfish algo (and std lib) to hash a byte array */
// hash will hash the given byte sequence using the bcrypt hash algorithm
func hash(password []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, hashCostValue)
}

// hashCost will hash the given byte sequence using the bcrypt algorithm with a cost parameter innit
func hashCost(password []byte, cost int) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, cost)
}

// verifyPass will use the bcrypt library to compare two byte sequences to match
func verifyPass(hashedPass, password []byte) bool {
	if err := bcrypt.CompareHashAndPassword(hashedPass, password); err == nil {
		return true
	}
	return false
}
