package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
)

func checkIfandExecBuiltIn(cmd string) error {

	// Handle ctrl + l
	// Handle exit signals/interrupts

	switch cmd {
	case builtInCommands[0].Name:
		os.Exit(0)
		return nil

	case builtInCommands[1].Name:
		usage()
		return nil

	default:
		return &CommandError{
			completeFlag: 0,
			message:      "notfound",
		}
	}

}

func getShellPrompt() string {
	// Who am I ?
	var (
		err      error
		whoami   string
		hostname string
	)

	output, err := user.Current()
	whoami = output.Username
	if err != nil {
		fmt.Println("Error retrieving username", err)
		whoami = "$NONAME$"
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
		usercolor, hostcolor, signcolor = White, White, White
	}

	fmt.Printf("%s%s[%s]%s ", ColorText(whoami, usercolor), delimitter, ColorText(hostname, hostcolor), ColorText(shell_sign, signcolor))

	buf := bufio.NewReader(os.Stdin)

	var line []byte
	for line, _, err = buf.ReadLine(); IsEmpty(string(line)); {
		fmt.Printf("Line scanned %s\n", string(line))
		if err != nil {
			fmt.Print("Error scanning input from stdin buffer\n", err)
		}
	}
	// Trim the input
	input := Trim(string(line))

	// Should Provide methods for new line extentions.

	return input

}

func welcome() {

	printStars := func(amount int) {
		fmt.Print("\n")
		for range amount {
			fmt.Print("*")
		}
		fmt.Print("\n")
	}

	printStars(30)

	fmt.Print(ColorText("***", Yellow) + " Welcome to the " + ColorText("gShell", Red) + "! " + ColorText("***", Yellow))

	printStars(30)

	fmt.Print("\n\n")
	usage()
}

func usage() {
	fmt.Print("List of Commands supported:\n")

	for _, v := range builtInCommands {
		v.printSelf()
	}

}

func Run() string {

	welcome()
	for {

		//fmt.Printf("i: %v\n", i)
		cmd := getShellPrompt()
		err := checkIfandExecBuiltIn(cmd)
		if err == nil {
			// If there is no err (meaning it exists)
			// continue safely
			continue
		}

		// Exec command
		// Check if piped // Perhaps check for malicious input?

	}

	fmt.Printf("%v", len("H E L L"))

	return "all smooth"

}
