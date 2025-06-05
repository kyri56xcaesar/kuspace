package shell

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"

	"kyri56xcaesar/kuspace/playground/logger"
	"kyri56xcaesar/kuspace/playground/shell/cosmetics"
)

const (
	// Conf paths
	defaultConfName = "config"
	alt1ConfName    = "gshell.conf"
	alt2ConfName    = "gconf.conf"

	defaultConfPath = "configs/"
	alt1ConfPath    = "~/.gshell/"
	alt2ConfPath    = "~/.config/gshell/"

	// Logs
	defaultLogPath     = "logfile.log"
	defaultInfologPath = "infofile.log"
	defaultErrlogPath  = "errfile.log"
	defaultWarnlogPath = "warnfile.log"

	// Cosmetics
	defaultShellSign  = "k_G_s>"
	defaultDelimitter = "<-?->"
)

// Sconfig as in Shell Config
var Sconfig SConfig

// SPrompt as in the data of the displayed line of the Shell prompt
type SPrompt struct {
	sign       string
	delimitter string

	datetime string

	gitbranch string
	gitstatus string
	dtActive  bool
	gitActive bool
}

// SCosmetics as in the variable data regarding display style
type SCosmetics struct {
	// text

	// colors
	usercolor string
	hostcolor string
	signcolor string
	prompt    SPrompt

	colorred bool
}

// Config FIELDS
// logFile path [universal]
// infoFile path [only info]
// warnFile path [only warnings]
// errFile path [only errors]
// colorred bool
// usercolor string
// hostcolor string
// signcolor string
// sign string
// delimitter string
// datetime string
// gitbranch string
// gitstatus string

// SConfig as in the configuration variables of the Shell
type SConfig struct {
	// logger info
	logFilePath string
	infoLogPath string
	warnLogPath string
	errLogPath  string

	theme      SCosmetics
	logVerbose bool
	logSplit   bool
	logLevel   int
}

func (sconfig *SConfig) setDefaults() {
	// visuals
	// prompt
	sconfig.theme = SCosmetics{
		prompt: SPrompt{
			sign:       defaultShellSign,
			delimitter: defaultDelimitter,
			datetime:   "",
			gitbranch:  "",
			gitstatus:  "",
		},
		colorred:  false,
		usercolor: "",
		hostcolor: "",
		signcolor: "",
	}

	// loggers
	sconfig.logLevel = 0
	sconfig.logVerbose = true
	sconfig.logSplit = false
	sconfig.logFilePath = defaultLogPath
	sconfig.errLogPath = defaultErrlogPath
	sconfig.infoLogPath = defaultInfologPath
	sconfig.warnLogPath = defaultWarnlogPath
}

func (sconfig *SConfig) setDefaultColors() {
	sconfig.theme.usercolor = cosmetics.Magenta
	sconfig.theme.hostcolor = cosmetics.Cyan
	sconfig.theme.signcolor = cosmetics.Green
}

func newConfig() SConfig {
	return SConfig{}
}

// LoadConfig loads config from a file and sets all the variables required
func LoadConfig() SConfig {
	Sconfig.setDefaults()
	// Should Look for the Config files in order
	// Priority
	// Lets work with default for now.
	configFile, err := os.Open(defaultConfPath + defaultConfName)
	if err != nil {
		// it means there is an error opening the file
		logger.Printf("Error opening config file at %s", defaultConfPath+defaultConfName)
		return Sconfig
	}
	defer func() {
		err := configFile.Close()
		if err != nil {
			logger.Printf("failed to close file: %v", err)
		}
	}()

	scanner := bufio.NewScanner(configFile)
	for scanner.Scan() {
		line := scanner.Text()

		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			logger.Printf("Invalid line config file %s", line)
			return Sconfig
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "del":
			if value != "" {
				Sconfig.theme.prompt.delimitter = value
			}
		case "sign":
			if value != "" {
				Sconfig.theme.prompt.sign = value
			}
		case "color":
			bvalue, err := strconv.ParseBool(value)
			if err == nil {
				Sconfig.theme.colorred = bvalue
				if bvalue {
					Sconfig.setDefaultColors()
				}
			}
		case "dt":
			bvalue, err := strconv.ParseBool(value)
			if err == nil {
				Sconfig.theme.prompt.dtActive = bvalue
				if bvalue {
					Sconfig.theme.prompt.datetime = time.Now().Format("HH:MM:SS")
				}
			}

		case "git":
			bvalue, err := strconv.ParseBool(value)
			if err == nil {
				Sconfig.theme.prompt.gitActive = bvalue
				//if bvalue {
				//}
			}
		case "log":
			Sconfig.logFilePath = value
		case "log_split":
			bvalue, err := strconv.ParseBool(value)
			if err == nil {
				Sconfig.logSplit = bvalue
			}
		case "log_level":
			ivalue, err := strconv.ParseInt(value, 10, 8)
			if err == nil {
				Sconfig.logLevel = int(ivalue)
			}
		case "log_verbose":
			bvalue, err := strconv.ParseBool(value)
			if err == nil {
				Sconfig.logVerbose = bvalue
			}
		case "err":
			Sconfig.errLogPath = value
		case "warn":
			Sconfig.warnLogPath = value
		case "info":
			Sconfig.infoLogPath = value

		}
	}

	return Sconfig
}
