/*
 cosmetics and theme related utility code
*/

// Package utils includes cosmetic logic
package utils

const (
	// Reset text character
	Reset = "\033[0m"
	// Red text character
	Red = "\033[31m"
	// Green text character
	Green = "\033[32m"
	// Yellow text character
	Yellow = "\033[33m"
	// Blue text character
	Blue = "\033[34m"
	// Magenta text character
	Magenta = "\033[35m"
	// Cyan text character
	Cyan = "\033[36m"
	// Gray text character
	Gray = "\033[37m"
	// White text character
	White = "\033[97m"
)

// ColorText will apply the given color to the argument text
func ColorText(s string, color string) string {
	return "" + color + s + Reset
}
