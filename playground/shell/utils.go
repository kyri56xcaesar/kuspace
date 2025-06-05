// Package shell sada
// safgsaf
package shell

import (
	"strings"
	"unicode"
)

// Trim function keeps only letters and numbers
// safsa
func Trim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

// IsEmpty testetst
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == "\n"
}
