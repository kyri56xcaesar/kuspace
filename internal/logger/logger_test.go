package logger

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

// TestGetLogger tests the creation and setup of the MultiLogger singleton
func TestGetLogger(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "logger_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir) // Clean up

	// Define paths for log files in temp directory
	logFilePath := tmpDir + "/log.log"
	errFilePath := tmpDir + "/err.log"
	warnFilePath := tmpDir + "/warn.log"
	infoFilePath := tmpDir + "/info.log"

	// Create a logger instance
	mlogger := &MultiLogger{
		logFilePath:  logFilePath,
		errFilePath:  errFilePath,
		warnFilePath: warnFilePath,
		infoFilePath: infoFilePath,
	}

	// Initialize logger with split enabled
	err = createLogger(true, true)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	// Check that the logger files are created
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		t.Errorf("Expected log file to be created at %s", logFilePath)
	}
	if _, err := os.Stat(errFilePath); os.IsNotExist(err) {
		t.Errorf("Expected error file to be created at %s", errFilePath)
	}
	if _, err := os.Stat(warnFilePath); os.IsNotExist(err) {
		t.Errorf("Expected warn file to be created at %s", warnFilePath)
	}
	if _, err := os.Stat(infoFilePath); os.IsNotExist(err) {
		t.Errorf("Expected info file to be created at %s", infoFilePath)
	}
}

// TestLoggingOperations tests basic logging functions (Print, Info, Warn, Error)
func TestLoggingOperations(t *testing.T) {
	mlogger := GetLogger(true, true)

	// Use Info logger to test output
	mlogger.Infof("Info log test %s", "message")
	mlogger.Warnln("Warning log test")
	mlogger.Err("Error log test")

	// Check if logger is set to verbose and writes to os.Stderr and file simultaneously
	if mlogger.verbose != true {
		t.Errorf("Expected verbose to be true, got %v", mlogger.verbose)
	}
}

// TestDestroyLogger tests the destruction and cleanup of the logger
func TestDestroyLogger(t *testing.T) {
	mlogger := GetLogger(true, true)

	// Destroy logger and check for error
	err := mlogger.DestroyLogger()
	if err != nil && !errors.Is(err, os.ErrClosed) {
		t.Errorf("Expected no error or ErrClosed, got %v", err)
	}

	// Check if logger files are closed by attempting to write after destroy
	if mlogger.lfile != nil {
		if _, err := mlogger.lfile.Write([]byte("test")); err == nil {
			t.Error("Expected write error after closing lfile, but got none")
		}
	}
	if mlogger.efile != nil {
		if _, err := mlogger.efile.Write([]byte("test")); err == nil {
			t.Error("Expected write error after closing efile, but got none")
		}
	}
	if mlogger.wfile != nil {
		if _, err := mlogger.wfile.Write([]byte("test")); err == nil {
			t.Error("Expected write error after closing wfile, but got none")
		}
	}
	if mlogger.ifile != nil {
		if _, err := mlogger.ifile.Write([]byte("test")); err == nil {
			t.Error("Expected write error after closing ifile, but got none")
		}
	}
}

