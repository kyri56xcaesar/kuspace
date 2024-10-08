package shell

import (
	"strings"
	"unicode"
)

func Trim(s string) string {

	return strings.TrimFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == "\n"
}
