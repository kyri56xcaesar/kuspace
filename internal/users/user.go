package users

const (
	Ppath string = "/etc/passwd"
	Gpath string = "/etc/group"
	Spath string = "/etc/shadow"
)

type User struct {
	UID      int
	Username string
	Password Password
	GID      int    // primary group
	GECOS    string // more user info
	HomeDir  string
	Shell    string
}

func NewUser() User {
	return User{}
}

// Useradd program should create a new user

// Userdel should delete a user

// Usermod should modify user permissions
