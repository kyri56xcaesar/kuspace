package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
)

const (
	shell_sign string = "k_s>"
	delimitter string = "@"
)

var (
	colorred  bool   = true
	usercolor string = White
	hostcolor string = White
	signcolor string = White
)

func getShellPrompt() string {
	// Who am I ?
	var (
		err      error
		whoami   string
		hostname string
		input    []byte
	)

	res := exec.Command("whoami")
	out, err := res.Output()
	if err != nil {
		fmt.Println(err.Error())
		whoami = "$NONAME"
	} else {
		whoami = Trim(string(out))
	}

	hostname, err = os.Hostname()
	if err != nil {
		fmt.Println("Error retrieving hostname", err)
		hostname = "$NOHOST"
	}

	if colorred {
		usercolor = Magenta
		hostcolor = Cyan
		signcolor = Green

	} else {
		usercolor, hostcolor, signcolor = u.White, u.White, u.White
	}

	fmt.Printf("%s%s[%s]%s ", ColorText(whoami, usercolor), delimitter, ColorText(hostname, hostcolor), ColorText(shell_sign, signcolor))

	buf := bufio.NewReader(os.Stdin)

	for input, _, err = buf.ReadLine(); u.IsEmpty(string(input)); {
		fmt.Printf("Line scanned %s\n", string(input))
		if err != nil {
			fmt.Print("Error scanning input from stdin buffer\n", err)
		}
	}

	fmt.Printf("What you have input is: %v\n", string(input))

	return Trim(string(input))

}

func usage() {
	fmt.Print(`
		\n*** Welcome to my gShell HELP ***	
		\nList of Commands supported:
		\n\texit
		\n\thelp -h --help
	`)
}

func Exec() string {

	for true {

		//fmt.Printf("i: %v\n", i)
		getShellPrompt()

	}

	return "all smooth:

}
