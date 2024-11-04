package main

import (
	"kyri56xcaesar/myThesis/internal/logger"
	"kyri56xcaesar/myThesis/internal/users"
)

func main() {

	logger.SetMultiLogger(false, true)

	newUser := users.NewUser()

	newPass, err := users.NewPassword(newUser.Username, "test")
	if err != nil {

	}

	logger.Printf("%+v", newUser)

	newPass.InactivityPeriod = "2"

}
