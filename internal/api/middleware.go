package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func securityMiddleWare(c *gin.Context) {
	//if c.Request.Host != srv.Config.Addr() {
	//	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid host header"})
	//	return
	//}
	c.Header("X-Frame-Options", "DENY")
	c.Header("Content-Security-Policy", "default-src 'self'; connect-src *; font-src *; script-src-elem * 'unsafe-inline'; img-src * data:; style-src * 'unsafe-inline';")
	c.Header("X-XSS-Protection", "1; mode=block")
	c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
	c.Header("Referrer-Policy", "strict-origin")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Permissions-Policy", "geolocation=(),midi=(),sync-xhr=(),microphone=(),camera=(),magnetometer=(),gyroscope=(),fullscreen=(self),payment=()")
	c.Next()
}

func autoLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Print("Auto Login middleware...")
		access_token, err := c.Cookie("access_token")
		if err != nil || access_token == "" {
			log.Printf("missing access token: %v", err)
			c.Next()
			return
		}

		groups, err := c.Cookie("groups")
		if err != nil || groups == "" {
			log.Printf("missing groups cookie: %v", err)
			c.Next()
			return
		}
		// forward directly inside
		if strings.Contains(groups, "admin") {
			c.Redirect(http.StatusSeeOther, "/api/v1/verified/admin-panel")
		} else {
			c.Redirect(http.StatusSeeOther, "/api/v1/verified/dashboard")
		}
	}
}

func AuthMiddleware(group string) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Print("Through Auth middleware...")
		accessToken, err := c.Cookie("access_token")
		if err != nil {
			log.Printf("missing access token: %v", err)
			c.Redirect(http.StatusSeeOther, "/api/v1/login")
			c.Abort()
			return
		}
		//  Decode and verify the token (e.g., JWT validation)
		req, err := http.NewRequest(http.MethodGet, authServiceURL+authVersion+"/user/me", nil)
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
		defer response.Body.Close()

		type Info struct {
			Expires_at string `json:"expires_at"`
			Groups     string `json:"groups"`
			Issues_at  string `json:"issued_at"`
			User       string `json:"user"`
			Valid      string `json:"valid"`
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

		if info.Info.Valid == "false" {
			log.Printf("token not valid anymore...")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized, invalid token"})
			c.Abort()
			return
		}

		if !strings.Contains(group, info.Info.Groups) {
			log.Printf("access group not included")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
	}
}

// Utils
func isBrowser(userAgent string) bool {
	browsers := []string{"Mozilla", "Chrome", "Safari", "Edge", "Opera"}
	for _, browser := range browsers {
		if strings.Contains(userAgent, browser) {
			return true
		}
	}
	return false
}
