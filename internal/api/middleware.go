package api

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

var jwtSecret = []byte("")

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.StandardClaims
}

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

func authMiddleWare(allowedRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Print("authMiddleware, confirming identity...")

		tokenString, err := c.Cookie("auth_token")
		log.Print(tokenString)
		if err != nil {
			log.Printf("error getting auth_token from cookie: %v", err)
			c.HTML(http.StatusUnauthorized, "error.html", gin.H{
				"error": strconv.Itoa(http.StatusUnauthorized), "message": "Unauthorized.",
			})
			c.Abort()
			return
		}

		claims := &Claims{}

		// Parse the token using `jwt.ParseWithClaims`
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		log.Printf("Claims: %+v", claims)
		log.Print(token)

		if err != nil || !token.Valid {
			log.Printf("error parsing with claims or token invalid: %v, %v", claims, err)
			c.HTML(http.StatusUnauthorized, "error.html", gin.H{
				"error": strconv.Itoa(http.StatusUnauthorized), "message": "Unauthorized.",
			})
			c.Abort()
			return
		}

		log.Printf("claims: %+v", claims)

		if claims.Role != allowedRole {
			log.Printf("error not allowed role %v != %v : %v", allowedRole, claims.Role, allowedRole != claims.Role)
			c.HTML(http.StatusForbidden, "error.html", gin.H{
				"error": strconv.Itoa(http.StatusForbidden), "message": "Forbidden.",
			})
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		log.Print("user confirmed. Setting variables...")
		c.Next()
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
