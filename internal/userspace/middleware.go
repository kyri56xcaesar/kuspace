package userspace

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

/* A custom HTTP header parser
*
* AccessTarget=<filepath> <signature>
*
* <filepath>: the name of the file you wish to acccess
* (if it ends in /, it means its a directory)
*
* <signature>:your_user_id:[group_id,groupd_id,...]
* so the signature plainly is the user ID delimitted by ':'
* followed by the group ids (delimitted by commas).
*
*
* */
func BindAccessTarget(http_header string) (AccessClaim, error) {
	var ac AccessClaim
	parts := strings.SplitN(http_header, " ", 2)
	if len(parts) != 2 {
		return ac, fmt.Errorf("invalid header format")
	}

	target := parts[0]
	if !strings.HasSuffix(target, "/") {
		target += "/"
	}

	sig := parts[1]
	p := strings.SplitN(sig, ":", 2)
	if len(p) != 2 {
		return ac, fmt.Errorf("invalid signature format")
	}
	if p == nil || p[0] == "" || p[1] == "" {
		return ac, fmt.Errorf("nil parameters")
	}

	ac = AccessClaim{
		Uid:    p[0],
		Gids:   p[1],
		Target: target,
	}

	return ac, nil
}

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

		if !resource.IsOwner(ac) {
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
			if !resource.HasAccess(ac) {
				log.Printf("user has no read access upon this resource")
				c.JSON(http.StatusForbidden, gin.H{"error": "not allowed read access on resource"})
				c.Abort()
				return
			}
		case "w":
			if !resource.HasWriteAccess(ac) {
				log.Printf("user has no write access upon this resource")
				c.JSON(http.StatusForbidden, gin.H{"error": "not allowed write access on resource"})
				c.Abort()
				return
			}
		case "x":
			if !resource.HasExecutionAccess(ac) {
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
