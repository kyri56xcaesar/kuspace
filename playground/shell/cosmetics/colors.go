// Package cosmetics sagksaglas
package cosmetics

const (
	// Reset text character
	Reset string = "\033[0m"
	// Red text character
	Red string = "\033[31m"
	// Green text character
	Green string = "\033[32m"
	// Yellow text character
	Yellow string = "\033[33m"
	// Blue text character
	Blue string = "\033[34m"
	// Magenta text character
	Magenta string = "\033[35m"
	// Cyan text character
	Cyan string = "\033[36m"
	// Gray text character
	Gray string = "\033[37m"
	// White text character
	White string = "\033[97m"
)

// ColorText will apply the given color to the argument text
func ColorText(s string, color string) string {
	return "" + color + s + Reset
}
