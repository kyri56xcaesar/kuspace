package utils

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

/*
* This module will contain functions and methods usefull for the other apps in the entire project
*
* Whatever is and can be reusable should be included here.
* */

/* some Functional Programming in Go */
// map
type mapFunc[E any] func(E) E

func Map[S ~[]E, E any](s S, f mapFunc[E]) S {
	result := make(S, len(s))
	for i := range s {
		result[i] = f(s[i])
	}
	return result
}

// filter
type keepFunc[E any] func(E) bool

func Filter[S ~[]E, E any](s S, f keepFunc[E]) S {
	result := S{}
	for _, v := range s {
		if f(v) {
			result = append(result, v)
		}
	}
	return result
}

// reduce
type reduceFunc[E any] func(cur, next E) E

func Reduce[E any](s []E, init E, f reduceFunc[E]) E {
	cur := init
	for _, v := range s {
		cur = f(cur, v)
	}
	return cur
}

// util
func ToFloat64(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	default:
		return 0
	}
}

// helper function to determine if a value is empty
func IsEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Map:
		return v.Len() == 0
	case reflect.Array:
		return v.Len() == 0
	default:
		return v.IsZero() // general case for other types
		// this might not work for everything
	}
}

func MakeMapFrom(names []string, values []any) map[string]any {
	if len(names) != len(values) {
		return nil
	}

	m := make(map[string]any)
	for i, arg := range values {
		reflectV := reflect.ValueOf(arg)
		if !IsEmpty(reflectV) {
			m[names[i]] = arg
		}
	}

	return m
}

/* generic helpers*/
func ToSnakeCase(input string) string {
	var output []rune
	for i, r := range input {
		if i > 0 && r >= 'A' && r <= 'Z' {
			output = append(output, '_')
		}
		output = append(output, r)
	}

	return strings.ToLower(string(output))
}

func SplitToInt(input, seperator string) ([]int, error) {
	// split the input string by comma
	parts := strings.Split(input, seperator)

	// trim spaces and convert to int
	trimAndConvert := func(s string) (int, error) {
		trimmed := strings.TrimSpace(s)
		return strconv.Atoi(trimmed)
	}

	// apply the trimAndConvert function to each part
	result := make([]int, len(parts))
	for i, part := range parts {
		value, err := trimAndConvert(part)
		if err != nil {
			return nil, err
		}
		result[i] = value
	}

	return result, nil
}

// this func will read multiple files, "concat" them (essentially appending one after another)
// and write to a single output file.
func MergeFiles(outputFile string, inputLocation string, inputFiles []string) error {
	out, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer out.Close()

	for _, inpPath := range inputFiles {
		fmt.Println("Processing:", inpPath)

		inpFile, err := os.Open(inputLocation + inpPath)
		if err != nil {
			fmt.Println("Error opening file:", inpPath, err)
			continue
		}
		defer inpFile.Close()

		_, err = io.Copy(out, inpFile) // Append content
		if err != nil {
			fmt.Println("Error writing file:", err)
			return err
		}

		// Optionally add a separator (newline)
		out.WriteString("\n")
	}

	fmt.Println("Merged files into:", outputFile)
	return nil
}

// short error messaging funcs..
func NewError(msg string, args ...any) error {
	return fmt.Errorf("[ERROR] %s", fmt.Sprintf(msg, args...))
}
func NewWarning(msg string, args ...any) error {
	return fmt.Errorf("[WARNING] %s", fmt.Sprintf(msg, args...))
}
func NewInfo(msg string, args ...any) error {
	return fmt.Errorf("[INFO] %s", fmt.Sprintf(msg, args...))
}

func HasInvalidCharacters(s, chars string) bool {
	// Escape regex meta-characters to avoid pattern errors
	var escapedChars []string
	for _, c := range chars {
		escapedChars = append(escapedChars, regexp.QuoteMeta(string(c)))
	}
	pattern := "[" + strings.Join(escapedChars, "") + "]"
	re := regexp.MustCompile(pattern)
	return re.MatchString(s)
}
