package logger

import (
	"errors"
	"io"
	"log"
	"os"

	"kyri56xcaesar/kuspace/playground/shell/cosmetics"
)

// Should implement LOG levels...

// multilogger must be a singleton
// Reason to implement custom logger, is for both stderr and file handle mechs and perhaps split logs
var MLogger MultiLogger = MultiLogger{logger: log.Default()}

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

// This method will return either a multiwriter if the logger is verbose
// or just the writer itself
func (mlogger *MultiLogger) getWriter(file *os.File) io.Writer {
	if mlogger.verbose {
		return io.MultiWriter(os.Stderr, file)
	} else {
		return file
	}
}

func (mlogger *MultiLogger) Print(v ...any) {
	if mlogger.logger != nil && mlogger.lfile != nil {
		mlogger.logger.Print(v...)
	}
}

func (mlogger *MultiLogger) Println(v ...any) {
	if mlogger.logger != nil && mlogger.lfile != nil {
		mlogger.logger.Println(v...)
	}
}

func (mlogger *MultiLogger) Printf(format string, v ...any) {
	if mlogger.logger != nil && mlogger.lfile != nil {
		mlogger.logger.Printf(format, v...)
	}
}

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

// Error should exit afterwards
func (mlogger *MultiLogger) Errf(error string, v ...any) {
	// switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.efile == nil {
		return
	}

	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.efile))
	mlogger.logger.SetPrefix(cosmetics.ColorText("[ERROR]: ", cosmetics.Red))
	mlogger.logger.SetFlags(log.Llongfile | log.Ldate | log.Ltime)

	mlogger.logger.Printf(error, v...)
}

// Error should exit afterwards
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

// Error should exit afterwards
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

func (mlogger *MultiLogger) DestroyLogger() error {
	err1 := mlogger.lfile.Close()
	err2 := mlogger.efile.Close()
	err3 := mlogger.wfile.Close()
	err4 := mlogger.ifile.Close()

	mlogger.logger = nil
	mlogger.ready = false

	return errors.Join(err1, err2, err3, err4)
}

func GetMultiLogger(split bool, verbose bool) *MultiLogger {
	if !MLogger.ready {
		err := createLogger(split, verbose)
		if err != nil {
		}
		return &MLogger
	} else {
		return &MLogger
	}
}

func SetMultiLogger(split bool, verbose bool) {
	if !MLogger.ready {
		err := createLogger(split, verbose)
		if err != nil {
		}
	}
}

func (mlogger *MultiLogger) setLoggerVerbosity(v bool) {
	mlogger.verbose = v
}

func (mlogger *MultiLogger) setLoggerSplit(split bool) {
	mlogger.split = split
}

// default logger redirects
func Print(v ...any) {
	MLogger.Print(v...)
}

func Println(v ...any) {
	MLogger.Println(v...)
}

func Printf(format string, v ...any) {
	MLogger.Printf(format, v...)
}

func SetDefaultLogger() {
	MLogger.DestroyLogger()

	MLogger.logger = log.Default()
}
