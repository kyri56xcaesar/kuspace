package model

// Users, their passwords and groups (minioth's identity model, as kuspace sees it).

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

// User represents a system user.
// @Description Contains user metadata, credentials, and group memberships.
type User struct {
	// Username is the unique login name of the user.
	// @example johndoe
	Username string `json:"username" example:"johndoe"`

	// Info is optional user metadata or notes.
	Info string `json:"info,omitempty" example:"researcher in group A"`

	// Verified means if a user email has his email verified
	Verified bool `json:"verified,omitempty"`

	// Home is the user’s home directory path.
	Home string `json:"home,omitempty" example:"/home/johndoe"`

	// Shell is the user’s default shell.
	Shell string `json:"shell,omitempty" example:"/bin/bash"`

	// Password contains the user’s password settings and hash.
	Password Password `json:"password"`

	// Groups is a list of groups the user belongs to.
	Groups []Group `json:"groups,omitempty"`

	// UID is the user’s numeric ID.
	UID int64 `json:"uid,omitempty"`

	// Pgroup is the user’s primary group ID.
	Pgroup int64 `json:"pgroup,omitempty"`
}

// ToString method formats and returns the permission object to a string
func (u *User) ToString() string {
	return fmt.Sprintf(`
		Name: %s, UID: %d
	`, u.Username, u.UID,
	)
}

// UsersToString function is similar to ToString method but works for a slice of User objects
func UsersToString(users []User) string {
	res := make([]string, 0, len(users))
	for _, user := range users {
		res = append(res, user.Username)
	}

	return strings.Join(res, ",")
}

// Password represents user password data and policies.
type Password struct {
	// Hashpass is the hashed password.
	// @example $2a$10$7s5YfF7...
	Hashpass string `json:"hashpass" example:"$2a$10$7s5YfF7..."`

	// LastPasswordChange is the last time the password was changed.
	LastPasswordChange string `json:"lastPasswordChange,omitempty"`

	MinimumPasswordAge string `json:"minimumPasswordAge,omitempty"`
	MaximumPasswordAge string `json:"maxiumPasswordAge,omitempty"`
	WarningPeriod      string `json:"warningPeriod,omitempty"`
	InactivityPeriod   string `json:"inactivityPeriod,omitempty"`
	ExpirationDate     string `json:"expirationDate,omitempty"`
}

/* check password fields for allowed values...*/

// ValidatePassword method sanitizes the password object and checks if it is within limits
func (p *Password) ValidatePassword() error {
	pass := p.Hashpass

	// Validate Password Length
	if len(pass) < 8 {
		return fmt.Errorf("password length %d is too short: at least 8 characters (%w)", len(pass), ut.ErrInvalid)
	}

	// Validate Hashpass
	if pass == "" {
		return fmt.Errorf("the password is empty (%w)", ut.ErrInvalid)
	}

	var hasUpper bool
	var hasDigit bool

	for _, r := range pass {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("the password needs an uppercase letter (%w)", ut.ErrInvalid)
	}

	if !hasDigit {
		return fmt.Errorf("the password needs a digit (%w)", ut.ErrInvalid)
	}

	return nil
}

// Group represents a group of users in the system.
type Group struct {
	// Groupname is the name of the group.
	// @example researchers
	Groupname string `json:"groupname" example:"researchers"`

	// Users is an optional list of users in the group.
	Users []User `json:"users,omitempty" swaggerignore:"true"`

	// Gid is the numeric group ID.
	// @example 3001
	GID int64 `json:"gid" example:"3001"`
}

// PtrFields returns a slice of pointers to the Resource struct fields,
// in a specific order. This is useful for scanning database rows directly
// into the struct fields or for generic update operations.
func (g *Group) PtrFields() []any {
	return []any{&g.Groupname, &g.GID}
}

// ToString method formats and returns the permission object to a string
func (g *Group) ToString() string {
	return g.Groupname
}

// GroupsToString function is similar to ToString method but works for a slice of Group objects and affects only names
func GroupsToString(groups []Group) string {
	res := make([]string, 0, len(groups))
	for _, group := range groups {
		res = append(res, group.ToString())
	}

	return strings.Join(res, ",")
}

// GidsToString function is similar to ToString method but works for a slice of Group objects but affects only group ids
func GidsToString(groups []Group) string {
	res := make([]string, 0, len(groups))
	for _, group := range groups {
		res = append(res, strconv.FormatInt(group.GID, 10))
	}

	return strings.Join(res, ",")
}
