package userspace

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func serviceAuth(srv *UService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s_secret_claim := c.GetHeader("X-Service-Secret"); s_secret_claim != "" {
			if s_secret_claim == string(srv.config.ServiceSecret) {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "must provide service token"})
		c.Abort()
	}
}

func isOwner(srv *UService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
		if err != nil {
			log.Printf("failed to bind access-target: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
			c.Abort()
			return
		}

		// root user is owner of all
		if ac.Uid == "0" {
			return
		}

		// lets grab the resource existing permissions:
		resource, err := srv.dbh.GetResourceByFilepath(ac.Target)
		if err != nil {
			log.Printf("error retrieving resource: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve resouce"})
			c.Abort()
			return
		}

		if !resource.IsOwner(*ac) {
			c.JSON(http.StatusForbidden, gin.H{"error": "user does not own this file"})
			c.Abort()
			return
		}
	}
}

/* This middleware should precheck if a user can claim access according
*  to the destined mode upon a resource
*
* mode should be read/write/execute
 */
func hasAccessMiddleware(mode string, srv *UService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
		if err != nil {
			log.Printf("failed to bind access-target: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
			c.Abort()
			return
		}

		// root user has access to all
		if ac.Uid == "0" {
			return
		}

		// lets grab the resource existing permissions:
		resource, err := srv.dbh.GetResourceByFilepath(ac.Target)
		if err != nil {
			log.Printf("error retrieving resource: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve resouce"})
			c.Abort()
			return
		}

		switch mode {
		case "r":
			if !resource.HasAccess(*ac) {
				log.Printf("user has no read access upon this resource")
				c.JSON(http.StatusForbidden, gin.H{"error": "not allowed read access on resource"})
				c.Abort()
				return
			}
		case "w":
			if !resource.HasWriteAccess(*ac) {
				log.Printf("user has no write access upon this resource")
				c.JSON(http.StatusForbidden, gin.H{"error": "not allowed write access on resource"})
				c.Abort()
				return
			}
		case "x":
			if !resource.HasExecutionAccess(*ac) {
				log.Printf("user has no execution access upon this resource")
				c.JSON(http.StatusForbidden, gin.H{"error": "not allowed execution access on resource"})
				c.Abort()
				return
			}

		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "bad settup"})
			return
		}
	}
}
