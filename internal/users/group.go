package users

type Group struct {
	Name      string
	Passwrod  string
	GID       int
	GroupList []User
}
