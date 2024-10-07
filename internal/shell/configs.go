package shell

const (
	shell_sign string = "k_s>"
	delimitter string = "@"
)

var (
	colorred  bool   = true
	usercolor string = White
	hostcolor string = White
	signcolor string = White
)

func ColorText(s string, color string) string {
	return "" + color + s + Reset
}
