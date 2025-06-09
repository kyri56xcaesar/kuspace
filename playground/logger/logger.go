// Package logger defines logging logic
// just a custom logger
package logger

import (
	"errors"
	"io"
	"log"
	"os"

	"kyri56xcaesar/kuspace/playground/shell/cosmetics"
)

// Should implement LOG levels...

// MLogger the main default variable logger (multilogger)
// Reason to implement custom logger, is for both stderr and file handle mechs and perhaps split logs'
// multilogger must be a singleton
var MLogger = MultiLogger{logger: log.Default()}

// MultiLogger struct describing every logging data needed
type MultiLogger struct {
	logger       *log.Logger
	lfile        *os.File
	efile        *os.File
	wfile        *os.File
	ifile        *os.File
	logFilePath  string
	errFilePath  string
	infoFilePath string
	warnFilePath string
	ready        bool
	split        bool
	verbose      bool
}

// This method will exit with either a multiwriter if the logger is verbose
// or just the writer itself
func (mlogger *MultiLogger) getWriter(file *os.File) io.Writer {
	if mlogger.verbose {
		return io.MultiWriter(os.Stderr, file)
	}

	return file
}

// Print wrapper to the standard log file
func (mlogger *MultiLogger) Print(v ...any) {
	if mlogger.logger != nil && mlogger.lfile != nil {
		mlogger.logger.Print(v...)
	}
}

// Println Wrapper to the standard log file
func (mlogger *MultiLogger) Println(v ...any) {
	if mlogger.logger != nil && mlogger.lfile != nil {
		mlogger.logger.Println(v...)
	}
}

// Printf wrapper to the standard log file
func (mlogger *MultiLogger) Printf(format string, v ...any) {
	if mlogger.logger != nil && mlogger.lfile != nil {
		mlogger.logger.Printf(format, v...)
	}
}

// Infof as in fmt.Printf but in the infolog file
func (mlogger *MultiLogger) Infof(information string, v ...any) {
	// Switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.ifile == nil {
		return
	}
	// determine writer (if verbose or not)
	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.ifile))
	mlogger.logger.SetPrefix(cosmetics.ColorText("[INFO]: ", cosmetics.Green))
	mlogger.logger.SetFlags(log.Lshortfile | log.Ldate)

	mlogger.logger.Printf(information, v...)
}

// Infoln as in fmt.Println but in the infolog ifle
func (mlogger *MultiLogger) Infoln(v ...any) {
	// Switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.ifile == nil {
		return
	}
	// determine writer (if verbose or not)
	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.ifile))
	mlogger.logger.SetPrefix(cosmetics.ColorText("[INFO]: ", cosmetics.Green))
	mlogger.logger.SetFlags(log.Lshortfile | log.Ldate)

	mlogger.logger.Println(v...)
}

// Info as int fmt.Print but in the infolog file
func (mlogger *MultiLogger) Info(v ...any) {
	// Switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.ifile == nil {
		return
	}
	// determine writer (if verbose or not)
	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.ifile))
	mlogger.logger.SetPrefix(cosmetics.ColorText("[INFO]: ", cosmetics.Green))
	mlogger.logger.SetFlags(log.Lshortfile | log.Ldate)

	mlogger.logger.Print(v...)
}

// Warnf as in fmt.Printf but in the warnlog file
func (mlogger *MultiLogger) Warnf(warning string, v ...any) {
	// switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.wfile == nil {
		return
	}

	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.wfile))
	mlogger.logger.SetPrefix(cosmetics.ColorText("[WARNING]: ", cosmetics.Yellow))
	mlogger.logger.SetFlags(log.Lshortfile | log.Ldate)

	mlogger.logger.Printf(warning, v...)
}

// Warnln same as Println but writes to the warnlog
func (mlogger *MultiLogger) Warnln(v ...any) {
	// switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.wfile == nil {
		return
	}

	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.wfile))
	mlogger.logger.SetPrefix(cosmetics.ColorText("[WARNING]: ", cosmetics.Yellow))
	mlogger.logger.SetFlags(log.Lshortfile | log.Ldate)

	mlogger.logger.Println(v...)
}

// Warn writes to the warrlog
func (mlogger *MultiLogger) Warn(v ...any) {
	// switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.wfile == nil {
		return
	}

	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.wfile))
	mlogger.logger.SetPrefix(cosmetics.ColorText("[WARNING]: ", cosmetics.Yellow))
	mlogger.logger.SetFlags(log.Lshortfile | log.Ldate)

	mlogger.logger.Print(v...)
}

// Errf same as Errorf but writes to the errlog
func (mlogger *MultiLogger) Errf(err string, v ...any) {
	// switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.efile == nil {
		return
	}

	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.efile))
	mlogger.logger.SetPrefix(cosmetics.ColorText("[ERROR]: ", cosmetics.Red))
	mlogger.logger.SetFlags(log.Llongfile | log.Ldate | log.Ltime)

	mlogger.logger.Printf(err, v...)
}

// Errln writes to the errlog
func (mlogger *MultiLogger) Errln(v ...any) {
	// switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.efile == nil {
		return
	}

	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.efile))
	mlogger.logger.SetPrefix(cosmetics.ColorText("[ERROR]: ", cosmetics.Red))
	mlogger.logger.SetFlags(log.Llongfile | log.Ldate | log.Ltime)

	mlogger.logger.Println(v...)
}

// Err writes to the errlog
func (mlogger *MultiLogger) Err(v ...any) {
	// switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.efile == nil {
		return
	}

	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.efile))
	mlogger.logger.SetPrefix(cosmetics.ColorText("[ERROR]: ", cosmetics.Red))
	mlogger.logger.SetFlags(log.Llongfile | log.Ldate | log.Ltime)

	mlogger.logger.Print(v...)
}

func createLogger(split bool, verbose bool) error {
	MLogger = MultiLogger{
		ready:        false,
		split:        split,
		verbose:      verbose,
		logFilePath:  "log.log",
		errFilePath:  "err.log",
		warnFilePath: "warn.log",
		infoFilePath: "info.log",
	}
	var (
		lerr, eerr, werr, ierr error
		multiWriter            io.Writer
	)

	if MLogger.split {
		MLogger.efile, eerr = os.OpenFile(MLogger.errFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		MLogger.wfile, werr = os.OpenFile(MLogger.warnFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		MLogger.ifile, ierr = os.OpenFile(MLogger.infoFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	} else {
		MLogger.lfile, lerr = os.OpenFile(MLogger.logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		multiWriter = io.MultiWriter(MLogger.lfile, os.Stderr)
	}

	MLogger.logger = log.New(multiWriter, cosmetics.ColorText("[LOG]: ", cosmetics.Cyan), log.Ldate|log.Ltime|log.Lshortfile)

	MLogger.ready = true

	return errors.Join(lerr, eerr, werr, ierr)
}

// DestroyLogger
func (mlogger *MultiLogger) destroyLogger() error {
	err1 := mlogger.lfile.Close()
	err2 := mlogger.efile.Close()
	err3 := mlogger.wfile.Close()
	err4 := mlogger.ifile.Close()

	mlogger.logger = nil
	mlogger.ready = false

	return errors.Join(err1, err2, err3, err4)
}

// GetMultiLogger gets or creates a bew MLogger
// "Singleton"
func GetMultiLogger(split bool, verbose bool) *MultiLogger {
	if !MLogger.ready {
		err := createLogger(split, verbose)
		if err != nil {
			log.Printf("failed to create logger")

			return nil
		}
	}

	return &MLogger
}

// SetMultiLogger sets the Mlogger to be the main logger
func SetMultiLogger(split bool, verbose bool) {
	if !MLogger.ready {
		err := createLogger(split, verbose)
		if err != nil {
			log.Fatalf("failed to set multi logger: %v", err)
		}
	}
}

func (mlogger *MultiLogger) setLoggerVerbosity(v bool) {
	mlogger.verbose = v
}

func (mlogger *MultiLogger) setLoggerSplit(split bool) {
	mlogger.split = split
}

// Print prints to the Mlogger
// Wrapper function
func Print(v ...any) {
	MLogger.Print(v...)
}

// Println same as fmt.Println but for the Mlogger
// Wrapper function
func Println(v ...any) {
	MLogger.Println(v...)
}

// Printf same as fmt.Printf but for the MLogger
// Wrapper function
func Printf(format string, v ...any) {
	MLogger.Printf(format, v...)
}

// SetDefaultLogger sets the global variable to the default logger
func SetDefaultLogger() {
	err := MLogger.destroyLogger()
	if err != nil {
		log.Printf("failed to destroy logger: %v", err)
	}

	MLogger.logger = log.Default()
}
