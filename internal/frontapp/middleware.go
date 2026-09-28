package frontendapp

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// isHTTPS reports whether the client reached us over TLS, directly or via a
// proxy that terminates it.
func isHTTPS(c *gin.Context) bool {
	return c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

func securityMiddleWare(c *gin.Context) {
	c.Header("X-Frame-Options", "DENY")
	// 'unsafe-inline' scripts are still required by the inline onclick handlers
	// in the templates; drop it once the frontend no longer uses them.
	c.Header("Content-Security-Policy",
		"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data:; font-src 'self'; connect-src 'self' ws: wss:; "+
			"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
	// HSTS only means something over HTTPS; sent on plain-HTTP localhost it
	// pins every localhost site in the browser to HTTPS.
	if isHTTPS(c) {
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
	c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Permissions-Policy",
		"geolocation=(),midi=(),sync-xhr=(),microphone=(),camera=(),magnetometer=(),gyroscope=(),fullscreen=(self),payment=()")
	c.Next()
}

// setSessionCookie sets an httpOnly, SameSite=Strict cookie scoped to the API,
// Secure whenever the request came over HTTPS.
func setSessionCookie(c *gin.Context, name, value string, maxAge int) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(name, value, maxAge, "/api/v1/", "", isHTTPS(c), true)
}
