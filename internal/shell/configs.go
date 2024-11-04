package shell

import (
	"bufio"
	"kyri56xcaesar/myThesis/internal/logger"
	"kyri56xcaesar/myThesis/internal/shell/cosmetics"
	"os"
	"strconv"
	"strings"
	"time"
)

const (

	// Conf paths
	DEFAULT_conf_name string = "config"
	Alt1_conf_name    string = "gshell.conf"
	Alt2_conf_name    string = "gconf.conf"

	DEFAULT_conf_path string = "configs/"
	Alt1_conf_path    string = "~/.gshell/"
	Alt2_conf_path    string = "~/.config/gshell/"

	// Logs
	DEFAULT_logfile_path string = "logfile.log"
	DEFAULT_infolog_path string = "infofile.log"
	DEFAULT_errlog_path  string = "errfile.log"
	DEFAULT_warnlog_path string = "warnfile.log"

	// Cosmetics
	DEFAULT_shell_sign string = "k_G_s>"
	DEFAULT_delimitter string = "<-?->"
)

var Sconfig SConfig

type SPrompt struct {
	sign       string
	delimitter string

	dt_active bool
	datetime  string

	git_active bool
	gitbranch  string
	gitstatus  string
}

type SCosmetics struct {

	// text
	prompt SPrompt

	// colors
	colorred  bool
	usercolor string
	hostcolor string
	signcolor string
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

type SConfig struct {
	// logger info
	logVerbose  bool
	logSplit    bool
	logLevel    int
	logFilePath string
	infoLogPath string
	warnLogPath string
	errLogPath  string

	// cosmetics
	theme SCosmetics
}

func (sconfig *SConfig) setDefaults() {

	// visuals
	// prompt
	sconfig.theme = SCosmetics{
		prompt: SPrompt{
			sign:       DEFAULT_shell_sign,
			delimitter: DEFAULT_delimitter,
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
	sconfig.logFilePath = DEFAULT_logfile_path
	sconfig.errLogPath = DEFAULT_errlog_path
	sconfig.infoLogPath = DEFAULT_infolog_path
	sconfig.warnLogPath = DEFAULT_warnlog_path

}

func (sconfig *SConfig) setDefaultColors() {
	sconfig.theme.usercolor = cosmetics.Magenta
	sconfig.theme.hostcolor = cosmetics.Cyan
	sconfig.theme.signcolor = cosmetics.Green
}

func newConfig() SConfig {
	return SConfig{}
}

func LoadConfig() SConfig {

	Sconfig.setDefaults()

	// Should Look for the Config files in order
	// Priority
	// Lets work with default for now.

	configFile, err := os.Open(DEFAULT_conf_path + DEFAULT_conf_name)
	if err != nil {
		//it means there is an error opening the file
		logger.Printf("Error opening config file at %s", DEFAULT_conf_path+DEFAULT_conf_name)
		return Sconfig
	}
	defer configFile.Close()

	scanner := bufio.NewScanner(configFile)
	for scanner.Scan() {
		line := scanner.Text()

		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			logger.Printf("Invalid line config file %s", &line)
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
				Sconfig.theme.prompt.dt_active = bvalue
				if bvalue {
					Sconfig.theme.prompt.datetime = time.Now().Format("HH:MM:SS")
				}
			}

		case "git":
			bvalue, err := strconv.ParseBool(value)
			if err == nil {
				Sconfig.theme.prompt.git_active = bvalue
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
