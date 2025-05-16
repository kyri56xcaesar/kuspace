package uspace

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

/* A custom HTTP header parser
*
* AccessTarget=<what_signature> <who_signature>
*
* <what>:volume_id:volume_name:resource_path
*
*	the 'what_signature' is describes the volume_id, volume_name, resource path
* 	delimitted by ':'
*
* <who>:your_user_id:[group_id,groupd_id,...]
*
*	the 'who_signature' describes the user_id and the groupids
* 	delimitted by ':'
*	 	(group_ids (delimitted by commas)).
*
*
* */
func BindAccessTarget(http_header string) (ut.AccessClaim, error) {
	var (
		ac                              ut.AccessClaim = ut.AccessClaim{}
		vid, vname, resource, uid, gids string
		target                          string
	)

	parts := strings.SplitN(http_header, " ", 2)
	if len(parts) != 2 {
		return ac, fmt.Errorf("invalid header format:all")
	}

	what := parts[0]
	who := parts[1]

	// parse the who
	parts = strings.SplitN(who, ":", 2)
	if len(parts) != 2 {
		return ac, fmt.Errorf("invalid header format:who")
	}
	uid = parts[0]
	gids = parts[1]

	if uid == "" || gids == "" {
		return ac, fmt.Errorf("invalid header format:who:empty_fields")
	}

	// parse the what
	parts = strings.SplitN(what, ":", 3)
	if len(parts) != 3 {
		return ac, fmt.Errorf("invalid header format:what")
	}
	vid = parts[0]
	vname = parts[1]
	resource = parts[2]

	if resource == "" || (vid == "" && vname == "") {
		return ac, fmt.Errorf("invalid header:what:empty")
	}

	// handle the Target specifier
	if strings.HasPrefix(resource, "/") {
		target = resource
	} else if strings.HasPrefix(resource, "$") {
		//keywords, rids
		parts = strings.Split(resource, "=")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return ac, fmt.Errorf("invalid header target format: $[keyword]=[values]")
		}

		switch strings.TrimPrefix(parts[0], "$") {
		case "rids":
			target = strings.TrimSpace(parts[1])
			ac.HasKeyword = true
		default:
			return ac, fmt.Errorf("invalid header target format: unrecognised keyword")
		}
	} else {
		return ac, fmt.Errorf("invalid header target format: prefix '$' or '/'")
	}

	ac.Vid = vid
	ac.Vname = vname
	ac.Target = target
	ac.Uid = uid
	ac.Gids = gids

	log.Printf("[Middleware] Request header binded: \t(who) %v:%v \t(what) %v:%v:%v", ac.Uid, ac.Gids, ac.Vid, ac.Vname, ac.Target)

	return ac, nil
}

func bindHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
		if err != nil {
			log.Printf("failed to bind access-target: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
			c.Abort()
			return
		}
		// save variables to gin context
		c.Set("accessTarget", ac)
	}
}

/*
	Middleware that will check for the X-Service-Secret http(custom) header, which is meant to provide

authentication to service-to-service comms.
*/
func serviceAuth(srv *UService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s_secret_claim := c.GetHeader("X-Service-Secret"); s_secret_claim != "" {
			if s_secret_claim == string(srv.config.SERVICE_SECRET_KEY) {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "must provide service token"})
		c.Abort()
	}
}

// these funcs should work for multiple incoming data
// func isOwner(srv *UService) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
// 		if err != nil {
// 			log.Printf("failed to bind access-target: %v", err)
// 			c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
// 			c.Abort()
// 			return
// 		}

// 		log.Printf("ac binded: %+v", ac)

// 		// root user is owner of all
// 		if ac.Uid == "0" {
// 			return
// 		}

// 		if ac.HasKeyword {
// 			resources, err := srv.storage.Select("", "resources", "rid IN", ac.Target, 0)
// 			if err != nil {
// 				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
// 				c.Abort()
// 				return
// 			}

// 			for _, resource := range resources {
// 				res := resource.(ut.Resource)
// 				if !res.IsOwner(ac) {
// 					c.JSON(http.StatusForbidden, gin.H{"error": "user does not own this file"})
// 					c.Abort()
// 					return
// 				}
// 			}
// 		} else {
// 			// lets grab the resource existing permissions:
// 			resource, err := srv.storage.SelectOne("", "resources", "name = ", ac.Target)
// 			if err != nil {
// 				log.Printf("error retrieving resource: %v", err)
// 				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve resouce"})
// 				c.Abort()
// 				return
// 			}
// 			res := resource.(ut.Resource)

// 			if !res.IsOwner(ac) {
// 				c.JSON(http.StatusForbidden, gin.H{"error": "user does not own this file"})
// 				c.Abort()
// 				return
// 			}
// 		}

// 	}
// }

/* This middleware should precheck if a user can claim access according
*  to the destined mode() on a resource
*
* mode should be read/write/execute
 */
// func hasAccessMiddleware(mode string, srv *UService) gin.HandlerFunc {
// 	return func(c *gin.Context) {

// 		// get header
// 		ac_h, exists := c.Get("accessTarget")
// 		if !exists {
// 			log.Printf("access target header was not set properly")
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "access target header was not set correctly"})
// 			return
// 		}
// 		ac := ac_h.(ut.AccessClaim)

// 		// root user has access to all
// 		if ac.Uid == "0" {
// 			return
// 		}

// 		if ac.HasKeyword {

// 			resources, err := srv.storage.Select("", "resources", "rid IN", ac.Target, 0)
// 			if err != nil {
// 				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
// 				c.Abort()
// 				return
// 			}

// 			for _, resource := range resources {
// 				res := resource.(ut.Resource)
// 				switch mode {
// 				case "r":
// 					if !res.HasAccess(ac) {
// 						log.Printf("user has no read access upon this resource")
// 						c.JSON(http.StatusForbidden, gin.H{"error": "not allowed read access on resource"})
// 						c.Abort()
// 						return
// 					}
// 				case "w":
// 					if !res.HasWriteAccess(ac) {
// 						log.Printf("user has no write access upon this resource")
// 						c.JSON(http.StatusForbidden, gin.H{"error": "not allowed write access on resource"})
// 						c.Abort()
// 						return
// 					}
// 				case "x":
// 					if !res.HasExecutionAccess(ac) {
// 						log.Printf("user has no execution access upon this resource")
// 						c.JSON(http.StatusForbidden, gin.H{"error": "not allowed execution access on resource"})
// 						c.Abort()
// 						return
// 					}

// 				default:
// 					c.JSON(http.StatusInternalServerError, gin.H{"error": "bad settup"})
// 					return
// 				}
// 			}
// 		} else {
// 			// lets grab the resource existing permissions:
// 			resource, err := srv.storage.SelectOne("", "resources", "name = ", ac.Target)
// 			if err != nil {
// 				log.Printf("error retrieving resource: %v", err)
// 				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve resouce"})
// 				c.Abort()
// 				return
// 			}

// 			res := resource.(ut.Resource)

// 			switch mode {
// 			case "r":
// 				if !res.HasAccess(ac) {
// 					log.Printf("user has no read access upon this resource")
// 					c.JSON(http.StatusForbidden, gin.H{"error": "not allowed read access on resource"})
// 					c.Abort()
// 					return
// 				}
// 			case "w":
// 				if !res.HasWriteAccess(ac) {
// 					log.Printf("user has no write access upon this resource")
// 					c.JSON(http.StatusForbidden, gin.H{"error": "not allowed write access on resource"})
// 					c.Abort()
// 					return
// 				}
// 			case "x":
// 				if !res.HasExecutionAccess(ac) {
// 					log.Printf("user has no execution access upon this resource")
// 					c.JSON(http.StatusForbidden, gin.H{"error": "not allowed execution access on resource"})
// 					c.Abort()
// 					return
// 				}

// 			default:
// 				c.JSON(http.StatusInternalServerError, gin.H{"error": "bad settup"})
// 				return
// 			}
// 		}

// 	}
// }
