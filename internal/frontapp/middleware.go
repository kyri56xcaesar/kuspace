package frontendapp

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

		//  Decode and verify the token (e.g., JWT validation)
		req, err := http.NewRequest(http.MethodGet, authServiceURL+authVersion+"/user/token", nil)
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

		type Info struct {
			ExpiresAt string `json:"expires_at"`
			Groups    string `json:"groups"`
			IssuesAt  string `json:"issued_at"`
			User      string `json:"user"`
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
		//  Decode and verify the token (e.g., JWT validation)
		req, err := http.NewRequest(http.MethodGet, authServiceURL+authVersion+"/user/token", nil)
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
		type Info struct {
			ExpiresAt string `json:"expires_at"`
			Groups    string `json:"groups"`
			GroupIDs  string `json:"groupIDs"`
			IssuesAt  string `json:"issued_at"`
			Username  string `json:"username"`
			UserID    string `json:"userID"`
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

		if info.Info.Valid == "false" {
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
				c.Set("groupIDs", info.Info.GroupIDs)
				c.Set("accessToken", accessToken)
				return
			}
		}

		log.Printf("access group not included")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
	}
}
