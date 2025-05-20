// Package utils provides a collection of reusable utility functions and helpers
// for use across the project. This package includes generic functional programming
// constructs (Map, Filter, Reduce), type conversion utilities, string manipulation,
// file operations, error formatting, and various validation helpers.
//
// Functional Programming Utilities:
//   - Map, Filter, Reduce: Generic implementations for slice processing.
//
// Type Conversion and Reflection:
//   - ToFloat64: Converts various numeric types to float64.
//   - IsEmpty: Checks if a value is the zero value for its type.
//   - MakeMapFrom: Constructs a map from two slices, omitting empty values.
//
// String Manipulation:
//   - ToSnakeCase: Converts CamelCase strings to snake_case.
//   - SplitToInt: Splits a string into a slice of integers.
//
// File Operations:
//   - MergeFiles: Concatenates multiple files into a single output file.
//   - ReadFileAt: Reads a specific byte range from a file.
//   - TailFileLines: Reads the last N lines from a file.
//
// Error Formatting:
//   - NewError, NewWarning, NewInfo: Formats error messages with severity tags.
//
// Slices:
//   - Contains
//
// Validation Helpers:
//   - HasInvalidCharacters: Checks for invalid characters in a string.
//   - IsNumeric, IsAlphanumeric, IsAlphanumericPlus: Validates string content.
//   - IsValidUTF8String: Validates if a string contains only allowed UTF-8 characters.
//
// Miscellaneous:
//   - CurrentTime: Returns the current UTC time in a standard format.
//   - SizeInGb: Converts a size in bytes to gigabytes.
//
// This package is intended to centralize commonly used logic and promote code reuse
// throughout the project.
package utils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

/*
* This module will contain functions and methods usefull for the other apps in the entire project
*
* Whatever is and can be reusable should be included here.
* */

/* some Functional Programming in Go */
// map
type mapFunc[E any, R any] func(E) R

func Map[S ~[]E, E any, R any](s S, f mapFunc[E, R]) []R {
	result := make([]R, len(s))
	for i, e := range s {
		result[i] = f(e)
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
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Array, reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		// Recursively check each field
		for i := 0; i < v.NumField(); i++ {
			if !isZeroValue(v.Field(i)) {
				return false
			}
		}
		return true
	default:
		return v.IsZero() // general case for other types
		// this might not work for everything
	}
}

func IsEmpty(val any) bool {
	if val == nil {
		return true
	}
	return isZeroValue(reflect.ValueOf(val))
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

func ReadFileAt(filePath string, start, end int64) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	if start >= fileSize {
		return nil, NewError("requested range exceeds file size")
	}
	if end >= fileSize {
		end = fileSize - 1
	}

	_, err = file.Seek(start, io.SeekStart)
	if err != nil {
		return nil, err
	}

	data := make([]byte, end-start+1)
	_, err = file.Read(data)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return data, nil
}

var time_format string = "2006-01-02 15:04:05-07:00"

func CurrentTime() string {
	return time.Now().UTC().Format(time_format)
}

// Security related utils
func IsNumeric(s string) bool {
	re := regexp.MustCompile(`^[0-9]+$`)
	return re.MatchString(s)
}

func IsAlphanumeric(s string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	return re.MatchString(s)
}

func IsAlphanumericPlus(s string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9@_]+$`)
	return re.MatchString(s)
}

func IsValidUTF8String(s string) bool {
	// Updated regex to include space (\s) and new line (\n) characters
	re := regexp.MustCompile(`^[\p{L}\p{N}\s\n!@#\$%\^&\*\(\):\?><\.\-]+$`)

	return re.MatchString(s)
}

func SizeInGb(s int64) float64 {
	return float64(s) / 1000000000
}

func TailFileLines(path string, n int) ([]string, error) {
	const readBlockSize = 4096

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	var (
		fileSize  = stat.Size()
		buffer    bytes.Buffer
		lineCount = 0
		offset    int64
		tmp       = make([]byte, readBlockSize)
	)

	for offset = fileSize; offset > 0 && lineCount <= n; {
		blockSize := int64(readBlockSize)
		if offset < blockSize {
			blockSize = offset
		}
		offset -= blockSize
		_, err := file.Seek(offset, io.SeekStart)
		if err != nil {
			return nil, err
		}
		readBytes := tmp[:blockSize]
		nr, err := file.Read(readBytes)
		if err != nil {
			return nil, err
		}

		buffer.Write(readBytes[:nr])
		lineCount += bytes.Count(readBytes[:nr], []byte{'\n'})
	}

	content := buffer.Bytes()
	lines := make([]string, 0, n)
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		text := scanner.Text()
		if utf8.ValidString(text) {
			lines = append(lines, text)
		} else {
			lines = append(lines, string(bytes.Runes([]byte(text))))
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

func Contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func ValidateObjectName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("object name cannot be empty")
	}

	// if strings.Contains(name, "/") {
	// 	return errors.New("object name cannot contain slashes")
	// }

	if strings.Contains(name, "..") {
		return errors.New("object name cannot contain '..'")
	}

	// Optional: disallow special characters (Windows-style)
	illegalChars := regexp.MustCompile(`[\\:*?"<>|]`)
	if illegalChars.MatchString(name) {
		return errors.New("object name contains illegal characters")
	}

	// Optional: enforce max length
	if len(name) > 255 {
		return errors.New("object name is too long")
	}

	// Optional: ensure it has an extension
	// if !strings.Contains(name, ".") {
	// 	return errors.New("object name must include an extension (e.g., .txt)")
	// }

	return nil
}
