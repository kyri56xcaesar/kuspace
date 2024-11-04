package users

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"kyri56xcaesar/myThesis/internal/logger"
)

// Use bcrypt hashes for passwords

const (
	cost int = 8
)

type Password struct {
	Username string
	Password []byte // Password Hash

	PasswordLChange string // password latest change
	MinAge          string // Will configure dateformat
	MaxAge          string

	InactivityPeriod string
}

func (p *Password) debug_Print() string {
	return fmt.Sprintf("Username: %s, Password: %s, LastChange: %s, MinAge: %s, MaxAge: %s, InactivePeriod: %s", p.Username, string(p.Password), p.PasswordLChange, p.MinAge, p.MaxAge, p.InactivityPeriod)
}

func NewPassword(user string, pass string) (Password, error) {

	logger.Print("Creating a new password")
	logger.Printf("User: %s, Pass: %s", user, pass)

	newPass := Password{
		Username: user,
	}

	bpass := []byte(pass)

	var err error
	newPass.Password, err = bcrypt.GenerateFromPassword(bpass, cost)
	if err != nil {
		logger.Printf("Failed to hash given password")
		return newPass, err
	}

	// FIll rest data of password

	logger.Print(newPass.debug_Print())

	return newPass, nil

}
