package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/user"

	"kyri56xcaesar/myThesis/internal/logger"
	"kyri56xcaesar/myThesis/internal/shell/cosmetics"
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
		logger.Printf("Error retrieving username", err)
		whoami = "$NONAME$"
	}

	hostname, err = os.Hostname()
	if err != nil {
		logger.Println("Error retrieving hostname", err)
		hostname = "$NOHOST"
	}

	fmt.Printf("%s%s[%s]%s ", cosmetics.ColorText(whoami, Sconfig.theme.usercolor), Sconfig.theme.prompt.delimitter, cosmetics.ColorText(hostname, Sconfig.theme.hostcolor), cosmetics.ColorText(Sconfig.theme.prompt.sign, Sconfig.theme.signcolor))

	buf := bufio.NewReader(os.Stdin)

	var line []byte
	for line, _, err = buf.ReadLine(); IsEmpty(string(line)); {
		logger.Printf("Line scanned %s\n", string(line))
		if err != nil {
			logger.Print("Error scanning input from stdin buffer\n", err)
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

	fmt.Print(cosmetics.ColorText("***", cosmetics.Yellow) + " Welcome to the " + cosmetics.ColorText("gShell", cosmetics.Red) + "! " + cosmetics.ColorText("***", cosmetics.Yellow))

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

func Run() error {

	LoadConfig()

	logger.SetMultiLogger(Sconfig.logSplit, Sconfig.logVerbose)

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

	return nil

}
