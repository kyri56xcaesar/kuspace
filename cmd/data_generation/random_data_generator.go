// Package main provides a command-line utility for generating random tabular data
// in various formats such as CSV, JSON, YAML, and plain text. The generator allows
// customization of the number of rows, columns, field length, character set, and
// output format. It supports writing the generated data to a file or printing it
// to standard output, and includes options for verbosity and operation timeout.
// This tool is useful for creating synthetic datasets for testing, development,
// or demonstration purposes.
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"
)

const (
	// MaxFold defines the number of times an occurence of a character appears in a block
	MaxFold = 20
	// MaxLength defines the number of characters in one line
	MaxLength = 1000
	// MaxHeight defines the number of lines
	MaxHeight         = 1000000000
	outputPath        = "tmp/"
	charsetPlus       = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()€¥"
	numbers           = "0123456789"
	characters        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	capsCharacters    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerCharacters   = "abcdefghijklmnopqrstuvwxyz"
	specialCharacters = "!@#$%^&*()€¥"
)

var (
	charset    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" //default charset
	output     = flag.String("out", "random_dataset.csv", "specify output file")
	lineLength = flag.Int("length", 10, "length of each field")
	lineHeight = flag.Int("lines", 10, "number of lines (rows)")
	format     = flag.String("format", "csv", "output format: txt, csv, json, yaml")
	fold       = flag.Int("fold", 3, "number of columns (fields per row)")

	verbose = flag.Bool("v", false, "print more details about what's happening")

	timeout = flag.Int64("timeout", 900, "timeout in seconds")
	toFile  = flag.Bool("f", false, "write output to file")

	nums    = flag.Bool("numbers", false, "charset only numbers")
	chars   = flag.Bool("chars", false, "charset characters only")
	caps    = flag.Bool("caps", false, "charset caps characters only")
	lower   = flag.Bool("lower", false, "charset lower characters only")
	special = flag.Bool("special", false, "charset special characters only")
)

// Row is a string to string map
type Row map[string]string

// Generates a 2D slice of random strings [rows][columns]
func generateRandomTable() [][]string {
	if *verbose {
		fmt.Println("Generating random data table")
	}
	data := make([][]string, *lineHeight)
	for i := 0; i < *lineHeight; i++ {
		row := make([]string, *fold)
		for j := 0; j < *fold; j++ {
			b := make([]byte, *lineLength)
			for k := range b {
				b[k] = charset[rand.Intn(len(charset))]
			}
			row[j] = string(b)
		}
		data[i] = row
	}
	return data
}

// Converts the table to the requested output format

func formatTo(formatType string, table [][]string) []byte {
	var out strings.Builder

	// Construct headers
	headers := make([]string, *fold)
	for i := 0; i < *fold; i++ {
		header := make([]byte, *fold)
		for k := 0; k < *fold; k++ {
			header[k] = charset[rand.Intn(len(charset))]
		}
		headers[i] = fmt.Sprintf("header-%v", string(header))
	}

	// Build slice of maps
	rows := make([]Row, len(table))
	for i, row := range table {
		rowMap := make(Row)
		for j, val := range row {
			rowMap[headers[j]] = val
		}
		rows[i] = rowMap
	}

	switch formatType {
	case "csv":
		writer := csv.NewWriter(&out)
		_ = writer.Write(headers)
		_ = writer.WriteAll(table)
		writer.Flush()
		return []byte(out.String())

	case "json":
		data, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			return []byte("error: failed to marshal JSON\n")
		}
		return data

	case "yaml", "yml":
		data, err := yaml.Marshal(rows)
		if err != nil {
			return []byte("error: failed to marshal YAML\n")
		}
		return data

	case "txt", "text", "str", "string":
		for _, row := range table {
			out.WriteString(strings.Join(row, "") + "\n")
		}
		return []byte(out.String())

	default:
		return []byte("Unsupported format\n")
	}
}

func writeToFile(filename string, data []byte) error {
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		log.Printf("failed to check-make file parents: %v", err)
		return err
	}
	f, err := os.OpenFile(outputPath+filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		log.Printf("failed to open file: %v", err)
		return err
	}
	_, err = f.Write(data)
	if err != nil {
		log.Printf("failed to write data to the file: %v", err)
	}

	err = f.Close()
	if err != nil {
		log.Printf("failed to close file: %v", err)
	}

	return err
}

func usage() {
	fmt.Println("Usage of random data generator:")
	flag.PrintDefaults()
}

func perform(ctx context.Context) {
	if *verbose {
		fmt.Println("Performing data generation...")
	}
	table := generateRandomTable()
	formatted := formatTo(*format, table)

	select {
	case <-ctx.Done():
		log.Println("Operation timed out.")
		return
	default:
		if *toFile {
			fmt.Println("Writing to file...")
			if err := writeToFile(*output, formatted); err != nil {
				log.Println("Write error:", err)
			}
		} else {
			if *verbose {
				fmt.Println("Printing to stdout...")
			}
			fmt.Print(string(formatted))
		}
	}
}

func parseFlags() {
	flag.Usage = usage
	flag.Parse()

	if *fold >= MaxFold {
		*fold = MaxFold
	} else if *fold <= 0 {
		*fold = 1
	}

	if *lineHeight >= MaxHeight {
		*lineHeight = MaxHeight
	} else if *lineHeight <= 0 {
		*lineHeight = 1
	}

	if *lineLength >= MaxLength {
		*lineLength = MaxLength
	} else if *lineLength <= 0 {
		*lineLength = 1
	}

	if *nums {
		charset = numbers
	} else if *chars {
		charset = characters
	} else if *caps {
		charset = capsCharacters
	} else if *lower {
		charset = lowerCharacters
	} else if *special {
		charset = specialCharacters
	}
}

func main() {

	parseFlags()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	if *verbose {
		fmt.Printf("f: %v, format: %v, length: %v, lines: %v, fold: %v, out: %v, timeout: %v\n",
			*toFile, *format, *lineLength, *lineHeight, *fold, *output, *timeout)
	}
	perform(ctx)
}
