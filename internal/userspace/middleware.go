package userspace

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	ut "kyri56xcaesar/myThesis/internal/utils"
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
	var ac AccessClaim = AccessClaim{}
	parts := strings.SplitN(http_header, " ", 2)
	if len(parts) != 2 {
		return ac, fmt.Errorf("invalid header format")
	}

	log.Printf("header: %s", http_header)
	var target string

	if strings.HasPrefix(parts[0], "/") {
		target = parts[0]
	} else if strings.HasPrefix(parts[0], "$") {
		//keywords, rids
		parts2 := strings.Split(parts[0], "=")
		if len(parts2) != 2 || parts2[0] == "" || parts2[1] == "" {
			return ac, fmt.Errorf("invalid header target format: $[keyword]=[values]")
		}

		switch strings.TrimPrefix(parts2[0], "$") {
		case "rids":
			target = strings.TrimSpace(parts2[1])
			ac.Keyword = true
		default:
			return ac, fmt.Errorf("invalid header target format: unrecognised keyword")
		}
	} else {
		return ac, fmt.Errorf("invalid header target format: prefix '$' or '/'")
	}

	issuerSignature := parts[1]
	p := strings.SplitN(issuerSignature, ":", 2)
	if len(p) != 2 {
		return ac, fmt.Errorf("invalid header signature format: [uid]:[gids]")
	}
	if p == nil || p[0] == "" || p[1] == "" {
		return ac, fmt.Errorf("nil parameters")
	}

	ac.Uid = p[0]
	ac.Gids = p[1]
	ac.Target = target

	log.Printf("ac: %+v", ac)

	return ac, nil
}

/*
	Middleware that will check for the X-Service-Secret http(custom) header, which is meant to provide

authentication to service-to-service comms.
*/
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

// these funcs should work for multiple incoming data
func isOwner(srv *UService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
		if err != nil {
			log.Printf("failed to bind access-target: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
			c.Abort()
			return
		}

		log.Printf("ac binded: %+v", ac)

		// root user is owner of all
		if ac.Uid == "0" {
			return
		}

		if ac.Keyword {
			target_int, err := ut.SplitToInt(ac.Target, ",")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				c.Abort()
				return
			}
			resources, err := srv.dbh.GetResourcesByIds(target_int)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				c.Abort()
				return
			}

			for _, resource := range resources {
				if !resource.IsOwner(ac) {
					c.JSON(http.StatusForbidden, gin.H{"error": "user does not own this file"})
					c.Abort()
					return
				}
			}
		} else {
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
}

/* This middleware should precheck if a user can claim access according
*  to the destined mode() on a resource
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

		log.Printf("ac binded: %+v", ac)

		// root user has access to all
		if ac.Uid == "0" {
			return
		}

		if ac.Keyword {
			target_int, err := ut.SplitToInt(ac.Target, ",")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				c.Abort()
				return
			}
			resources, err := srv.dbh.GetResourcesByIds(target_int)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				c.Abort()
				return
			}

			for _, resource := range resources {
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
		} else {
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
}
