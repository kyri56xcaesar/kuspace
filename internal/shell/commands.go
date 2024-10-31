package shell

import (
	"fmt"
)

type BCommand struct {
	Name        string
	Description string
	Alias       []string
	Flags       []string
}

func (bc *BCommand) printSelf() {

	formatFlags := func() (string, int) {
		if len(bc.Flags) == 0 {
			return "", 0
		}

		flagsStr := fmt.Sprintf("%v:", bc.Flags)

		return flagsStr, len(Trim(flagsStr))
	}

	f, l := formatFlags()

	formatTabs := func() string {
		if l <= 4 {
			return "\t\t"
		} else {
			return "\t"
		}

	}

	fmt.Printf("\t%s\t%s%s%s\n", bc.Name, f, formatTabs(), bc.Description)

}

type CommandError struct {
	completeFlag int
	message      string
}

func (cerr *CommandError) Error() string {
	return fmt.Sprintf("Error msg: %s -> Completion: %d", cerr.message, cerr.completeFlag)
}

var builtInCommands [4]BCommand = [4]BCommand{
	{
		Name:        "exit",
		Description: "Exit the shell",
		Flags:       nil,
		Alias:       []string{"quit", "exit()", "quit()"},
	},
	{
		Name:        "help",
		Description: "This message",
		Flags:       []string{"-h", "--help"},
		Alias:       []string{"h"},
	},
	{
		Name:        "clear",
		Description: "Clear the screen",
		Flags:       nil,
		Alias:       []string{"cls"},
	},
	{
		Name:        "history",
		Description: "Your history of commands in this session",
		Flags:       []string{"-a", "--all"},
		Alias:       []string{"hst"},
	},
}
