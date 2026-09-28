package frontendapp

import (
	"net/http"
	"strings"

	"kyri56xcaesar/kuspace/internal/authn"

	"github.com/gin-gonic/gin"
)

/*
	Sessions

	A session is minioth's access token in the HttpOnly, SameSite=Strict
	"accessToken" cookie. frontapp verifies it itself with the configured
	algorithm (internal/authn, the same check uspace does). Logging out or
	changing the password revokes the user's tokens at minioth; the verifier
	learns that from minioth's introspection, cached for 30 seconds.
*/

const sessionCookie = "accessToken"

// session returns the verified claims of the request's session cookie.
func (srv *HTTPService) session(c *gin.Context) (*authn.Claims, string, bool) {
	raw, err := c.Cookie(sessionCookie)
	if err != nil || raw == "" {
		return nil, "", false
	}
	claims, err := srv.verifier.Verify(raw)
	if err != nil {
		return nil, "", false
	}

	return claims, raw, true
}

// requireRole admits sessions whose user is in one of the named groups
// (exact names: the old check was a substring test, so a group called
// "adm" - or no group at all - passed as "admin").
func (srv *HTTPService) requireRole(groups ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, raw, ok := srv.session(c)
		if !ok {
			toLogin(c, http.StatusUnauthorized, "your session has expired, log in again")

			return
		}
		if !claims.InGroup(groups...) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not allowed for your account"})

			return
		}
		c.Set("claims", claims)
		c.Set("accessToken", raw)
		// the keys the handlers read
		c.Set("username", claims.Username)
		c.Set("userID", claims.UserID)
		c.Set("groups", claims.Groups)
		c.Set("groupIDs", claims.GIDs()) // primary group first
		c.Next()
	}
}

// toLogin ends a request that needs a session: pages redirect to the login
// page, htmx requests get HX-Redirect, API calls a JSON error.
func toLogin(c *gin.Context, status int, msg string) {
	switch {
	case c.GetHeader("HX-Request") == "true":
		c.Header("HX-Redirect", "/api/v1/login")
		c.AbortWithStatus(status)
	case c.Request.Method == http.MethodGet && strings.Contains(c.GetHeader("Accept"), "text/html"):
		c.Redirect(http.StatusSeeOther, "/api/v1/login")
		c.Abort()
	default:
		c.AbortWithStatusJSON(status, gin.H{"error": msg})
	}
}

// redirectIfLoggedIn sends a visitor with a valid session from the login
// page straight to the panel.
func (srv *HTTPService) redirectIfLoggedIn() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, _, ok := srv.session(c); ok {
			c.Redirect(http.StatusSeeOther, "/api/v1/verified/admin-panel")
			c.Abort()

			return
		}
		c.Next()
	}
}
