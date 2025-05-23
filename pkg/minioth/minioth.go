// Package minioth provides a core user and group management system with pluggable backends.
// It supports operations such as adding, deleting, modifying, and authenticating users and groups.
// The package is configurable via environment variables and supports both plain file and database handlers.
// Passwords are securely hashed using bcrypt, with a configurable hash cost.
// Minioth is designed to be extensible and integrates with custom utility types and configuration loading.
//
// WARNING: The plain handler is not recommended for production use due to security concerns.
package minioth

import (
	ut "kyri56xcaesar/kuspace/internal/utils"
	"log"
	"os"
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
	audit_log_path           = "data/logs/minioth/audit.log"
	log_path                 = "data/logs/minioth/minioth.log"
	forbidden_names []string = []string{
		"root",
		"kubernetes",
		"k8s",
		"admin",
		"manager",
		"superuser",
		"sudo",
	}
	HASH_COST int = 16
)

/*
*
* */
// Minioth is the core type of this system
type Minioth struct {
	handler MiniothHandler
	Service MService
	Config  ut.EnvConfig
}

/* each minioth instance consists of a handler which
*  implements this interface.
* */
//
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

	Close()
}

/* "constructor"
* Use this function to create an instance of minioth. */
// NewMinioth constructs a new minioth object and initializes drawing from a configuration file specified
func NewMinioth(cfgPath string) Minioth {
	log.Print("[INIT]Creating new minioth...")
	newM := Minioth{
		Config: ut.LoadConfig(cfgPath),
	}

	var err error
	HASH_COST, err = strconv.Atoi(newM.Config.HASH_COST)
	if err != nil {
		log.Fatalf("failed to atoi hash_cost var: %v", err)
	}

	audit_log_path = newM.Config.MINIOTH_AUDIT_LOGS
	log_path = newM.Config.API_LOGS_PATH

	log.Printf("[INIT]setting log path to: %s", log_path)
	log.Printf("[INIT]setting audit log path to: %s", audit_log_path)
	log.Printf("[INIT]setting hashcost to: HASH_COST=%v", HASH_COST)

	newM.handler = handlerFactory(&newM)
	newM.Service = NewMSerivce(&newM)
	newM.handler.Init()

	return newM
}

// handlerFactory will producde the chosen "storage" system specified in the configuration
func handlerFactory(minioth *Minioth) MiniothHandler {
	switch minioth.Config.MINIOTH_HANDLER {
	case "db", "database":
		return &DBHandler{DBpath: minioth.Config.MINIOTH_DB, minioth: minioth}
	case "plain", "text", "file":
		return &PlainHandler{minioth: minioth}
	default:
		log.Fatal("not a valid handler, cannot operate")
		return nil
	}
}

/* attach handler methods to minioth.
*  just to avoid redundant dot calls
* */
func (m *Minioth) Useradd(user ut.User) (int, int, error) {
	return m.handler.Useradd(user)
}

func (m *Minioth) Userdel(username string) error {
	return m.handler.Userdel(username)
}

func (m *Minioth) Usermod(user ut.User) error {
	return m.handler.Usermod(user)
}

func (m *Minioth) Userpatch(uid string, fields map[string]interface{}) error {
	return m.handler.Userpatch(uid, fields)
}

func (m *Minioth) Groupadd(group ut.Group) (int, error) {
	return m.handler.Groupadd(group)
}

func (m *Minioth) Groupdel(groupname string) error {
	return m.handler.Groupdel(groupname)
}

func (m *Minioth) Groupmod(group ut.Group) error {
	return m.handler.Groupmod(group)
}

func (m *Minioth) Grouppatch(gid string, fields map[string]interface{}) error {
	return m.handler.Grouppatch(gid, fields)
}

func (m *Minioth) Passwd(username, password string) error {
	return m.handler.Passwd(username, password)
}

func (m *Minioth) Select(id string) any {
	return m.handler.Select(id)
}

func (m *Minioth) Authenticate(username, password string) (ut.User, error) {
	return m.handler.Authenticate(username, password)
}

/* NOTE: irrelevant atm
* delete the 3 state files */
// Purge (equivalent to Destrutor) is supposed to destroy the object and its deps.
func (m *Minioth) Purge() {
	log.Print("Purging everything...")

	_, err := os.Stat("data/plain")
	if err == nil {
		log.Print("data/plain dir exist")

		err = os.Remove(MINIOTH_PASSWD)
		if err != nil {
			log.Print(err)
		}
		err = os.Remove(MINIOTH_GROUP)
		if err != nil {
			log.Print(err)
		}
		err = os.Remove(MINIOTH_SHADOW)
		if err != nil {
			log.Print(err)
		}
		err = os.Remove("data/plain")
		if err != nil {
			log.Print(err)
		}
	}

	_, err = os.Stat("data/db")
	if err == nil {
		log.Print("data/db dir exists")
		err = os.Remove("data/*.db")
		if err != nil {
			log.Print(err)
		}

		err = os.Remove("data/db")
		if err != nil {
			log.Print(err)
		}
	}
}

/* NOTE: irrelevant atm
* This function should sync the DB state and the Plain state. TODO:*/
// Sync is supposed to convert and check from one storage handler to another.
// WARNING. not implemented yet
func (m *Minioth) Sync() error {
	return nil
}

/* UTIL functions */
/* use bcrypt blowfish algo (and std lib) to hash a byte array */
// hash will hash the given byte sequence using the bcrypt hash algorithm
func hash(password []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, HASH_COST)
}

// hash_cost will hash the given byte sequence using the bcrypt algorithm with a cost parameter innit
func hash_cost(password []byte, cost int) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, cost)
}

// verifyPass will use the bcrypt library to compare two byte sequences to match
func verifyPass(hashedPass, password []byte) bool {
	if err := bcrypt.CompareHashAndPassword(hashedPass, password); err == nil {
		return true
	}
	return false
}
