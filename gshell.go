package main

import (
	"fmt"
	"os/exec"
)

const (
	shell_sign string = "k_s>"
	delimitter string = "@"
)

func getShellPrompt() string {
	// Who am I ?
	cmd := exec.Command("whoami")
	whoami, err := cmd.Output()
	if err != nil {
		fmt.Println(err.Error())
		return ""
	}

	// What is my hostname?
	hostname := "*placeholder*"

	fmt.Printf("%s%s%s%s ", whoami, delimitter, hostname, shell_sign)

	var input string

	fmt.Scanln(&input)

	return input

}

func main() {

	for true {

		//fmt.Printf("i: %v\n", i)
		cmd := getShellPrompt()

		fmt.Printf("What you have inputed is: %v\n", cmd)

		break

	}
}
