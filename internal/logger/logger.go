package logger

import (
	"errors"
	"io"
	"log"
	"os"

	"kyri56xcaesar/myThesis/internal/colors"
)

// multilogger must be a singleton
// Reason to implement custom logger, is for both stderr and file handle mechs and perhaps split logs
var logger MultiLogger

type MultiLogger struct {
	logger *log.Logger

	ready   bool
	split   bool
	verbose bool

	logFilePath string
	lfile       *os.File

	errFilePath string
	efile       *os.File

	warnFilePath string
	wfile        *os.File

	infoFilePath string
	ifile        *os.File
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
	mlogger.logger.SetPrefix(colors.ColorText("[INFO]: ", colors.Green))
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
	mlogger.logger.SetPrefix(colors.ColorText("[INFO]: ", colors.Green))
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
	mlogger.logger.SetPrefix(colors.ColorText("[INFO]: ", colors.Green))
	mlogger.logger.SetFlags(log.Lshortfile | log.Ldate)

	mlogger.logger.Print(v...)
}

func (mlogger *MultiLogger) Warnf(warning string, v ...any) {
	// switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.wfile == nil {
		return
	}

	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.wfile))
	mlogger.logger.SetPrefix(colors.ColorText("[WARNING]: ", colors.Yellow))
	mlogger.logger.SetFlags(log.Lshortfile | log.Ldate)

	mlogger.logger.Printf(warning, v...)

}

func (mlogger *MultiLogger) Warnln(v ...any) {
	// switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.wfile == nil {
		return
	}

	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.wfile))
	mlogger.logger.SetPrefix(colors.ColorText("[WARNING]: ", colors.Yellow))
	mlogger.logger.SetFlags(log.Lshortfile | log.Ldate)

	mlogger.logger.Println(v...)

}

func (mlogger *MultiLogger) Warn(v ...any) {
	// switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.wfile == nil {
		return
	}

	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.wfile))
	mlogger.logger.SetPrefix(colors.ColorText("[WARNING]: ", colors.Yellow))
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
	mlogger.logger.SetPrefix(colors.ColorText("[ERROR]: ", colors.Red))
	mlogger.logger.SetFlags(log.Llongfile | log.Ldate | log.Ltime)

	mlogger.logger.Printf(error, v...)
}

// Error should exit afterwards
func (mlogger *MultiLogger) Errln(error string, v ...any) {
	// switch writer if split
	if mlogger == nil || mlogger.logger == nil || mlogger.efile == nil {
		return
	}

	mlogger.logger.SetOutput(mlogger.getWriter(mlogger.efile))
	mlogger.logger.SetPrefix(colors.ColorText("[ERROR]: ", colors.Red))
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
	mlogger.logger.SetPrefix(colors.ColorText("[ERROR]: ", colors.Red))
	mlogger.logger.SetFlags(log.Llongfile | log.Ldate | log.Ltime)

	mlogger.logger.Print(v...)
}

func createLogger(split bool, verbose bool) error {

	logger = MultiLogger{
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

	if logger.split {
		logger.efile, eerr = os.OpenFile(logger.errFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		logger.wfile, werr = os.OpenFile(logger.warnFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		logger.ifile, ierr = os.OpenFile(logger.infoFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	} else {
		logger.lfile, lerr = os.OpenFile(logger.logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		multiWriter = io.MultiWriter(logger.lfile, os.Stderr)
	}

	logger.logger = log.New(multiWriter, colors.ColorText("[LOG]: ", colors.Cyan), log.Ldate|log.Ltime|log.Lshortfile)

	logger.ready = true

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

func GetLogger(split bool, verbose bool) *MultiLogger {

	if !logger.ready {
		err := createLogger(split, verbose)
		if err != nil {
		}
		return &logger
	} else {
		return &logger
	}
}
