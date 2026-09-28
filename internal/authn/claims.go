// Package authn verifies minioth access tokens for the kuspace services
// (uspace, frontapp) and exposes who the caller is.
package authn

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Claims are the minioth access-token claims the services rely on.
type Claims struct {
	UserID   string `json:"user_id"` //nolint:tagliatelle // minioth wire format (snake_case)
	Username string `json:"username"`
	Groups   string `json:"groups"`    // group names, comma separated
	GroupIDs string `json:"group_ids"` //nolint:tagliatelle // minioth wire format (snake_case)
	PGroup   string `json:"pgroup"`    // primary group id
	jwt.RegisteredClaims
}

// GIDs is the caller's group ids, primary group first (uspace gives new
// files the first group); the user id when there are none.
func (c *Claims) GIDs() string {
	gids := []string{}
	if c.PGroup != "" && c.PGroup != "0" {
		gids = append(gids, c.PGroup)
	}
	for _, g := range strings.Split(c.GroupIDs, ",") {
		if g = strings.TrimSpace(g); g != "" && g != c.PGroup {
			gids = append(gids, g)
		}
	}
	if len(gids) == 0 {
		gids = append(gids, c.UserID) // Access-Target needs at least one group
	}

	return strings.Join(gids, ",")
}

// Who renders the identity as Access-Target's "uid:gids".
func (c *Claims) Who() string {
	return c.UserID + ":" + c.GIDs()
}

// InGroup reports whether the caller belongs to any of the named groups.
// Names are compared exactly.
func (c *Claims) InGroup(names ...string) bool {
	for _, g := range strings.Split(c.Groups, ",") {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		for _, n := range names {
			if g == n {
				return true
			}
		}
	}

	return false
}

// IsAdmin reports membership of the "admin" group.
func (c *Claims) IsAdmin() bool {
	return c.InGroup("admin")
}
