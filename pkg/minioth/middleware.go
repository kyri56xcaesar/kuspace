package minioth

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware middleware function, for authorization control
/* For this service, authorization is required only for admin role. */
func AuthMiddleware(role string, srv *MService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// if service secret exists and validated, grant access
		if secretClaim := c.GetHeader("X-Service-Secret"); secretClaim != "" {
			if secretClaim == string(srv.Minioth.Config.ServiceSecretKey) {
				log.Printf("service secret accepted. access granted.")
				c.Next()

				return
			}
			log.Printf("service secret invalid. access not granted")
			c.Abort()

			return
		}
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()

			return
		}

		// Extract the token from the Authorization header
		tokenString := authHeader[len("Bearer "):]
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token is required"})
			c.Abort()

			return
		}

		// Parse and validate the token
		token, err := ParseJWT(tokenString)
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()

			return
		}

		// set claims in the context for further use
		if claims, ok := token.Claims.(*CustomClaims); ok {
			rg := strings.Split(role, ",")
			if len(rg) == 0 {
				log.Printf("user has no groups set...")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "token doesn't include a group"})
				c.Abort()

				return
			}
			ok = false
			for _, r := range rg {
				if strings.Contains(claims.Groups, r) {
					c.Set("uid", claims.Subject)
					c.Set("username", claims.Username)
					c.Set("groups", claims.Groups)
					c.Set("gids", claims.GroupIDs)
					ok = true

					break
				}
			}
			if !ok {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "invalid user",
				})
				c.Abort()

				return
			}
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			c.Abort()

			return
		}
	}
}
