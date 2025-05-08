package minioth

import (
	ut "kyri56xcaesar/myThesis/internal/utils"
	"log"
	"os"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

/*
* WARNING: don't use the plain handler.
 */

/* hash cost for bcrypt hash function, reconfigurable from config*/
var HASH_COST int = 16

/*
*
* */
type Minioth struct {
	handler MiniothHandler
	Service MService
	Config  ut.EnvConfig
}

/* each minioth instance consists of a handler which
*  implements this interface.
* */
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

	Select(id string) []any
	Passwd(username, password string) error
	Authenticate(username, password string) (*ut.User, error)

	Close()
}

/* "constructor"
* Use this function to create an instance of minioth. */
func NewMinioth(cfgPath string) Minioth {
	log.Print("Creating new minioth...")
	newM := Minioth{
		Config: ut.LoadConfig(cfgPath),
	}
	newM.handler = handlerFactory(&newM)
	newM.Service = NewMSerivce(&newM)
	newM.handler.Init()

	var err error
	HASH_COST, err = strconv.Atoi(newM.Config.HASH_COST)
	if err != nil {
		log.Fatalf("failed to atoi hash_cost var: %v", err)
	}

	return newM
}

func handlerFactory(minioth *Minioth) MiniothHandler {
	switch minioth.Config.MINIOTH_HANDLER {
	case "db", "database":
		return &DBHandler{DBpath: minioth.Config.MINIOTH_DB, minioth: minioth}
	case "plain", "text", "txt":
		return &PlainHandler{}
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

func (m *Minioth) Select(id string) []interface{} {
	return m.handler.Select(id)
}

func (m *Minioth) Authenticate(username, password string) (*ut.User, error) {
	return m.handler.Authenticate(username, password)
}

/* NOTE: irrelevant atm
* delete the 3 state files */
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
func (m *Minioth) Sync() error {
	return nil
}

/* UTIL functions */
/* use bcrypt blowfish algo (and std lib) to hash a byte array */
func hash(password []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, HASH_COST)
}

func hash_cost(password []byte, cost int) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, cost)
}

/* check if a passowrd is correct */
func verifyPass(hashedPass, password []byte) bool {
	if err := bcrypt.CompareHashAndPassword(hashedPass, password); err == nil {
		return true
	}
	return false
}
