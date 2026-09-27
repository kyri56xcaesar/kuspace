package frontendapp

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

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

func autoLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet || c.Request.URL.Path != "/login" {
			c.Next()

			return
		}
		log.Print("Auto Login middleware...")
		accessToken, err := c.Cookie("accessToken")
		if err != nil || accessToken == "" {
			log.Printf("missing access token: %v", err)
			c.Next()

			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
		defer cancel()
		//  Decode and verify the token (e.g., JWT validation)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, authServiceURL+authVersion+"/user/token", nil)
		if err != nil {
			log.Printf("failed to create a new req: %v", err)
			c.JSON(http.StatusInternalServerError, nil)
			c.Abort()

			return
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(req)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to validate access"})
			c.Abort()

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()

		if response.StatusCode != http.StatusOK {
			log.Printf("token introspection failed: status %v", response.Status)
			c.Next()

			return
		}

		type Info struct {
			ExpiresAt string `json:"expires_at"` //nolint:tagliatelle // minioth wire format (snake_case)
			Groups    string `json:"groups"`
			IssuesAt  string `json:"issued_at"` //nolint:tagliatelle // minioth wire format (snake_case)
			User      string `json:"username"`
			Valid     string `json:"valid"`
		}
		var info struct {
			Info Info `json:"info"`
		}
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to read resp body",
			})
			c.Abort()

			return
		}

		err = json.Unmarshal(body, &info)
		if err != nil {
			log.Printf("failed to unmarshal response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to parse response",
			})
			c.Abort()

			return
		}

		if info.Info.Valid != "true" {
			c.Next()

			return
		}

		// forward directly inside
		c.Redirect(http.StatusSeeOther, "/api/v1/verified/admin-panel")
	}
}

func authMiddleware(group string) gin.HandlerFunc {
	return func(c *gin.Context) {
		accessToken, err := c.Cookie("accessToken")
		if err != nil {
			log.Printf("missing access token: %v", err)
			c.HTML(401, "admin-panel.html", gin.H{"error": "token has expired, login again"})
			c.Abort()

			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
		defer cancel()
		//  Decode and verify the token (e.g., JWT validation)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, authServiceURL+authVersion+"/user/token", nil)
		if err != nil {
			log.Printf("failed to create a new req: %v", err)
			c.JSON(http.StatusInternalServerError, nil)
			c.Abort()

			return
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(req)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to validate access"})
			c.Abort()

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()

		// minioth v1.0.0 (third_party/minioth) returns 400 with {"error": ...}
		// for an invalid/expired token instead of 200 with {"info":{"valid":
		// "false"}}. This check must come before decoding: without it, decoding
		// an error body into the zero-valued Info struct below yields an empty
		// Groups string, and strings.Contains(group, "") is always true -
		// silently granting access with a blank identity.
		if response.StatusCode != http.StatusOK {
			log.Printf("token introspection failed: status %v", response.Status)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized, invalid token"})
			c.Abort()

			return
		}

		type Info struct {
			ExpiresAt string `json:"expires_at"` //nolint:tagliatelle // minioth wire format (snake_case)
			Groups    string `json:"groups"`
			GroupIDs  string `json:"group_ids"` //nolint:tagliatelle // minioth wire format (snake_case)
			IssuesAt  string `json:"issued_at"` //nolint:tagliatelle // minioth wire format (snake_case)
			Username  string `json:"username"`
			UserID    string `json:"user_id"` //nolint:tagliatelle // minioth wire format (snake_case)
			PGroup    string `json:"pgroup"`
			Valid     string `json:"valid"`
		}
		var info struct {
			Info Info `json:"info"`
		}
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to read resp body",
			})
			c.Abort()

			return
		}

		err = json.Unmarshal(body, &info)
		if err != nil {
			log.Printf("failed to unmarshal response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to parse response",
			})
			c.Abort()

			return
		}

		// log.Printf("%+v", info)

		if info.Info.Valid != "true" {
			log.Printf("token not valid anymore...")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized, invalid token"})
			c.Abort()

			return
		}
		contents := strings.Split(info.Info.Groups, ",")
		for _, g := range contents {
			if strings.Contains(group, strings.TrimSpace(g)) {
				/* set this context value for the template rendering needed later*/
				c.Set("username", info.Info.Username)
				c.Set("userID", info.Info.UserID)
				c.Set("groups", info.Info.Groups)
				// primary group first: uspace gives new files the first group
				c.Set("groupIDs", primaryFirst(info.Info.GroupIDs, info.Info.PGroup))
				c.Set("accessToken", accessToken)

				return
			}
		}

		log.Printf("access group not included")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
	}
}

// primaryFirst reorders a comma-separated gid list so pgroup comes first
// (added if missing). An empty pgroup leaves the list unchanged.
func primaryFirst(gids, pgroup string) string {
	if pgroup == "" || pgroup == "0" {
		return gids
	}
	out := []string{pgroup}
	for _, g := range strings.Split(gids, ",") {
		if g = strings.TrimSpace(g); g != "" && g != pgroup {
			out = append(out, g)
		}
	}

	return strings.Join(out, ",")
}
