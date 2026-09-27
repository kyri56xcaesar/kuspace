package uspace

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

/*
	Who is calling uspace.

	uspace no longer takes the caller's identity on trust. Two kinds of caller:

	- users (frontapp on their behalf, a CLI, gshell, ...) send their minioth
	  access token as "Authorization: Bearer <jwt>". uspace verifies it itself
	  and the identity (uid, groups, primary group) comes from the token; any
	  identity in the Access-Target header is overwritten, so the header only
	  says *what* is being accessed.
	- services (frontapp's admin operations, background jobs) send the shared
	  X-Service-Secret. Only they may state an identity in Access-Target.

	Anything else is refused.
*/

// accessClaims are the minioth access-token claims uspace relies on.
type accessClaims struct {
	UserID   string `json:"user_id"` //nolint:tagliatelle // minioth wire format (snake_case)
	Username string `json:"username"`
	Groups   string `json:"groups"`
	GroupIDs string `json:"group_ids"` //nolint:tagliatelle // minioth wire format (snake_case)
	PGroup   string `json:"pgroup"`
	jwt.RegisteredClaims
}

var errNoCredentials = errors.New("no credentials")

// tokenVerifier returns the service's verifier (tests build UService
// without the constructor).
func (srv *UService) tokenVerifier() *tokenVerifier {
	if srv.tokens == nil {
		srv.tokens = newTokenVerifier(srv.config)
	}

	return srv.tokens
}

// who renders the token identity as Access-Target's "uid:gids", primary
// group first (uspace gives new files the first group).
func (c *accessClaims) who() string {
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

	return c.UserID + ":" + strings.Join(gids, ",")
}

func (c *accessClaims) isAdmin() bool {
	for _, g := range strings.Split(c.Groups, ",") {
		if strings.TrimSpace(g) == "admin" {
			return true
		}
	}

	return false
}

func bearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}

	return ""
}

// identify resolves the caller. For a user token it rewrites Access-Target so
// its identity part is the token's; a service keeps the header as sent.
func identify(srv *UService, c *gin.Context) (*accessClaims, bool, error) {
	if raw := bearerToken(c); raw != "" {
		claims, err := srv.tokenVerifier().verify(raw)
		if err != nil {
			return nil, false, fmt.Errorf("invalid token: %w", err)
		}
		what := "0::/"
		if cur := c.GetHeader("Access-Target"); cur != "" {
			what, _, _ = strings.Cut(cur, " ")
		}
		c.Request.Header.Set("Access-Target", what+" "+claims.who())

		return claims, false, nil
	}
	if s := c.GetHeader("X-Service-Secret"); s != "" && s == string(srv.config.ServiceSecretKey) {
		return nil, true, nil
	}

	return nil, false, errNoCredentials
}

// authenticate lets through users with a valid access token and services
// with the service secret.
func authenticate(srv *UService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, _, err := identify(srv, c); err != nil {
			log.Printf("[Middleware-Auth] refused: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "send a minioth access token or the service secret"})

			return
		}
		c.Next()
	}
}

// authenticateAdmin lets through services and users in the admin group.
func authenticateAdmin(srv *UService) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, service, err := identify(srv, c)
		switch {
		case err != nil:
			log.Printf("[Middleware-Auth] refused admin call: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "send a minioth access token or the service secret"})
		case !service && !claims.isAdmin():
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin only"})
		default:
			c.Next()
		}
	}
}
