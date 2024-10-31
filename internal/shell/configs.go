package shell

const (

	// Conf paths
	DEFAULT_conf_name string = "config"
	Alt1_conf_name    string = "gshell.conf"
	Alt2_conf_name    string = "gconf.conf"

	DEFAULT_conf_path string = "../../configs/"
	Alt1_conf_path    string = "~/.gshell/"
	Alt2_conf_path    string = "~/.config/gshell/"

	// Logs
	DEFAULT_logfile_path string = "logfile.log"
	DEFAULT_infolog_path string = "infofile.log"
	DEFAULT_errlog_path  string = "errfile.log"
	DEFAULT_warnlog_path string = "warnfile.log"

	// Cosmetics
	DEFAULT_shell_sign string = "k_G_s>"
	DEFAULT_delimitter string = "<-?^?->"
)

var (
	Sconfig SConfig = LoadConfig()
)

type SPrompt struct {
	sign       string
	delimitter string
	datetime   string
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

type SConfig struct {
	// logger info
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
	sconfig.logFilePath = DEFAULT_logfile_path
	sconfig.errLogPath = DEFAULT_errlog_path
	sconfig.infoLogPath = DEFAULT_infolog_path
	sconfig.warnLogPath = DEFAULT_warnlog_path

}

func newConfig() SConfig {
	return SConfig{}
}

func LoadConfig() SConfig {

	sconfig := newConfig()
	sconfig.setDefaults()

	// Should Look for the Config files in order
	// Priority
	// Lets work with default for now.

	//configFile, err := os.Open(DEFAULT_conf_path + DEFAULT_conf_name)
	//if err != nil {
	//	// it means there is an error opening the file
	//}

	return sconfig
}
