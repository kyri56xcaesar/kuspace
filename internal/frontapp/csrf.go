package frontendapp

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

/*
	CSRF protection (double-submit cookie).

	Every visitor gets a random csrf_token cookie that pages can read
	(web/static/js/csrf.js); every state-changing request must send it back
	in X-CSRF-Token. Another site can make the browser send the cookie but
	can't read it, so it can't set the header. The session cookie is also
	SameSite=Strict; this covers browsers or setups where that falls short.
	A request whose Origin names another host is refused as well.
*/

const (
	csrfCookie = "csrf_token"
	csrfHeader = "X-Csrf-Token"
)

func newCSRFToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}

// csrf issues the token cookie and checks state-changing requests. exempt
// paths are those that can't carry a token (the login and registration
// forms, which run before any session exists).
func csrf(exempt ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(csrfCookie)
		if err != nil || len(token) != 64 {
			token = newCSRFToken()
			c.SetSameSite(http.SameSiteStrictMode)
			// readable by the page's JS on purpose (that's the mechanism)
			c.SetCookie(csrfCookie, token, 0, "/", "", isHTTPS(c), false)
		}
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()

			return
		}
		if !sameOrigin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross-site request refused"})

			return
		}
		for _, p := range exempt {
			if c.Request.URL.Path == p {
				c.Next()

				return
			}
		}
		sent := c.GetHeader(csrfHeader)
		if sent == "" || subtle.ConstantTimeCompare([]byte(sent), []byte(token)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing or stale security token, reload the page"})

			return
		}
		c.Next()
	}
}

// sameOrigin rejects requests whose Origin (or, lacking it, Referer) names
// a different host than the one serving the request.
func sameOrigin(c *gin.Context) bool {
	src := c.GetHeader("Origin")
	if src == "" || src == "null" {
		src = c.GetHeader("Referer")
	}
	if src == "" {
		return true // non-browser clients; the token check still applies
	}
	u, err := url.Parse(src)
	if err != nil {
		return false
	}

	return strings.EqualFold(u.Host, c.Request.Host)
}
