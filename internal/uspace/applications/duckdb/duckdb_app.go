package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"time"

	"database/sql"

	_ "github.com/marcboeker/go-duckdb"
)

var (
	validFormats = []string{"json", "csv", "parquet", "txt"}
)

const (
	DEFAULT_TIMEOUT = 60  // 1h
	MAX_TIMEOUT     = 600 // 10h
)

type EnvVars struct {
	INPUT         string
	OUTPUT        string
	OUTPUT_FORMAT string
	INPUT_FORMAT  string
	LOGIC         string

	TIMEOUT int
}

func newEnvVar() *EnvVars {
	timeout := os.Getenv("TIMEOUT")
	t_int, err := strconv.Atoi(timeout)
	if err != nil {
		log.Printf("failed to atoi timeout as env variable, setting to default")
		t_int = DEFAULT_TIMEOUT
	}

	if t_int > MAX_TIMEOUT {
		t_int = MAX_TIMEOUT
	}

	return &EnvVars{
		INPUT:         os.Getenv("INPUT"),
		OUTPUT:        os.Getenv("OUTPUT"),
		OUTPUT_FORMAT: os.Getenv("OUTPUT_FORMAT"),
		INPUT_FORMAT:  os.Getenv("INPUT_FORMAT"),
		LOGIC:         os.Getenv("LOGIC"),
		TIMEOUT:       t_int,
	}
}

func isValidFormat(format string) bool {
	return slices.Contains(validFormats, format)
}

func (e *EnvVars) Validate() error {
	if e.INPUT == "" {
		return fmt.Errorf("INPUT is not set")
	}
	if e.OUTPUT == "" {
		return fmt.Errorf("OUTPUT is not set")
	}
	if e.LOGIC == "" {
		return fmt.Errorf("LOGIC is not set")
	}
	if e.OUTPUT_FORMAT == "" {
		e.OUTPUT_FORMAT = "json"
	}
	if e.INPUT_FORMAT == "" {
		e.INPUT_FORMAT = "json"
	}

	if !isValidFormat(e.OUTPUT_FORMAT) {
		return fmt.Errorf("invalid OUTPUT_FORMAT: %s", e.OUTPUT_FORMAT)
	}

	if !isValidFormat(e.INPUT_FORMAT) {
		return fmt.Errorf("invalid INPUT_FORMAT: %s", e.INPUT_FORMAT)
	}

	return nil
}

func main() {
	// get env variables
	env := newEnvVar()

	// validate env var
	if err := env.Validate(); err != nil {
		fmt.Printf("error validating env vars: %v\n", err)
		os.Exit(1)
	}

	// handle input perhaps?

	// perform logic
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		log.Fatalf("failed to connect to DuckDB: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Duration(env.TIMEOUT).Minutes()))
	defer cancel()

	res, err := db.QueryContext(ctx, env.LOGIC, nil)
	if err != nil {
		log.Fatalf("failed to execute query: %v", err)
	}
	defer res.Close()
	// handle output

	// yea this seems like a lot of work..
	// lets stick with python for nwo for this task
}
