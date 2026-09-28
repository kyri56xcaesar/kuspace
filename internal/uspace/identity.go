package uspace

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"kyri56xcaesar/kuspace/internal/authn"
	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
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

var errNoCredentials = errors.New("no credentials")

// ctxPrivileged is set by authenticate: the caller is a service or an admin.
const ctxPrivileged = "privileged"

// privileged reports whether the caller may act on everyone's jobs: a
// service, an admin, or root.
func privileged(c *gin.Context, ac ut.AccessClaim) bool {
	return c.GetBool(ctxPrivileged) || ac.UID == "0"
}

// tokenVerifier returns the service's verifier (tests build UService
// without the constructor).
func (srv *UService) tokenVerifier() *authn.Verifier {
	if srv.tokens == nil {
		srv.tokens = authn.NewVerifier(srv.config)
	}

	return srv.tokens
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
func identify(srv *UService, c *gin.Context) (*authn.Claims, bool, error) {
	if raw := bearerToken(c); raw != "" {
		claims, err := srv.tokenVerifier().Verify(raw)
		if err != nil {
			return nil, false, fmt.Errorf("invalid token: %w", err)
		}
		what := "0::/"
		if cur := c.GetHeader("Access-Target"); cur != "" {
			what = cur // a bare "what", or "what who" (the target may contain spaces)
			if i := strings.LastIndex(cur, " "); i >= 0 && whoPattern.MatchString(cur[i+1:]) {
				what = cur[:i]
			}
		}
		c.Request.Header.Set("Access-Target", what+" "+claims.Who())

		return claims, false, nil
	}
	if s := c.GetHeader("X-Service-Secret"); s != "" && len(srv.config.ServiceSecretKey) > 0 &&
		subtle.ConstantTimeCompare([]byte(s), srv.config.ServiceSecretKey) == 1 {
		return nil, true, nil
	}

	return nil, false, errNoCredentials
}

// authenticate lets through users with a valid access token and services
// with the service secret.
func authenticate(srv *UService) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, service, err := identify(srv, c)
		if err != nil {
			log.Printf("[Middleware-Auth] refused: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "send a minioth access token or the service secret"})

			return
		}
		c.Set(ctxPrivileged, service || claims.IsAdmin())
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
		case !service && !claims.IsAdmin():
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin only"})
		default:
			c.Next()
		}
	}
}
