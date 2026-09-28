package uspace

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

// BindAccessTarget function, used by middleware
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
*	the 'who_signature' describes the userID and the groupids
* 	delimitted by ':'
*	 	(groupIDs (delimitted by commas)).
*
*
* */
// whoPattern is Access-Target's identity part: "uid:gid[,gid...]".
var whoPattern = regexp.MustCompile(`^[0-9]+:[0-9]+(,[0-9]+)*$`)

// BindAccessTarget parses an Access-Target header, "vid:vname:target
// uid:gid[,gid...]" (see above), into an AccessClaim.
func BindAccessTarget(httpHeader string) (ut.AccessClaim, error) {
	var (
		ac                              = ut.AccessClaim{}
		vid, vname, resource, uid, gids string
		target                          string
	)

	// Split at the LAST space: the identity never contains one, the target
	// may. Splitting at the first one let a target like "/f.txt 0:0" (from
	// a download link) become the identity - uid 0, root.
	i := strings.LastIndex(httpHeader, " ")
	if i < 0 {
		return ac, errors.New("invalid header format:all")
	}
	what, who := httpHeader[:i], httpHeader[i+1:]
	if !whoPattern.MatchString(who) {
		return ac, errors.New("invalid header format:who")
	}

	var parts []string

	// parse the who
	parts = strings.SplitN(who, ":", 2)
	if len(parts) != 2 {
		return ac, errors.New("invalid header format:who")
	}
	uid = parts[0]
	gids = parts[1]

	if uid == "" || gids == "" {
		return ac, errors.New("invalid header format:who:empty_fields")
	}

	// parse the what
	parts = strings.SplitN(what, ":", 3)
	if len(parts) != 3 {
		return ac, errors.New("invalid header format:what")
	}
	vid = parts[0]
	vname = parts[1]
	resource = parts[2]

	if resource == "" || (vid == "" && vname == "") {
		return ac, errors.New("invalid header:what:empty")
	}

	// handle the Target specifier
	if strings.HasPrefix(resource, "/") {
		target = resource
	} else if strings.HasPrefix(resource, "$") {
		// keywords, rids
		parts = strings.Split(resource, "=")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return ac, errors.New("invalid header target format: $[keyword]=[values]")
		}

		switch strings.TrimPrefix(parts[0], "$") {
		case "rids":
			target = strings.TrimSpace(parts[1])
			ac.HasKeyword = true
		default:

			return ac, errors.New("invalid header target format: unrecognised keyword")
		}
	} else {
		target = "/" + resource
	}

	ac.VID = vid
	ac.Vname = vname
	ac.Target = target
	ac.UID = uid
	ac.Gids = gids

	return ac, nil
}

func bindHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
		if err != nil {
			log.Printf("[Middleware-HBinder] failed to bind access-target: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
			c.Abort()

			return
		}
		// save variables to gin context
		c.Set("accessTarget", ac)
		log.Printf("[Middleware] Request header binded: \t(who) %v:%v \t(what) %v:%v:%v\tkeyword?: %v",
			ac.UID, ac.Gids, ac.VID, ac.Vname, ac.Target, ac.HasKeyword)
	}
}

// errTargetNotFound: the Access-Target names no existing resource.
var errTargetNotFound = errors.New("resource not found")

// accessTargets returns the resources an Access-Target refers to: the listed
// rids for a "$rids=" keyword target, otherwise the one resource with exactly
// that name in that volume. (It used to match `name LIKE %target%` across all
// volumes, so a check could hit unrelated files - e.g. "o_x.txt" matched
// "hello_x.txt" - and underscores acted as wildcards.)
func accessTargets(ctx context.Context, srv *UService, ac ut.AccessClaim) ([]ut.Resource, error) {
	if ac.HasKeyword {
		res, err := srv.fsl.SelectObjects(ctx, map[string]any{"rids": strings.TrimPrefix(ac.Target, "/"), "vname": ac.Vname})
		if err != nil {
			return nil, err
		}
		resources, ok := res.([]ut.Resource)
		if !ok {
			return nil, errors.New("unexpected resource type")
		}
		if len(resources) == 0 {
			return nil, errTargetNotFound
		}

		return resources, nil
	}
	r, found, err := srv.lookupResource(ctx, ac.Target, ac.Vname)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errTargetNotFound
	}

	return []ut.Resource{r}, nil
}

// abortTargetError answers a failed accessTargets lookup.
func abortTargetError(c *gin.Context, tag string, err error) {
	if errors.Is(err, errTargetNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
	} else {
		log.Printf("[%s] failed to select access-target object(s): %v", tag, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
	c.Abort()
}

// these funcs should work for multiple incoming data
func isOwner(srv *UService) gin.HandlerFunc {
	return func(c *gin.Context) {
		access, exists := c.Get("accessTarget")
		if !exists {
			log.Printf("[Middleware-Ownership] failed to retrieve access-target header")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
			c.Abort()

			return
		}
		ac, ok := access.(ut.AccessClaim)
		if !ok {
			log.Printf("[Middleware-Ownership] failed to cast ac, :?corrupt data?, should have been set already")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "acces-target setup error"})
			c.Abort()

			return
		}
		// root user is owner and father of all
		if ac.UID == "0" {
			log.Printf("[Middleware-Ownership] root access detected, free pass...")
			c.Next()

			return
		}
		resources, err := accessTargets(c.Request.Context(), srv, ac)
		if err != nil {
			abortTargetError(c, "Middleware-Ownership", err)

			return
		}
		for _, resource := range resources {
			if !resource.IsOwner(ac) {
				log.Printf("[Middleware-Ownership] unauthorized access, user does not apply ownership on item)")
				c.JSON(http.StatusForbidden, gin.H{"error": "user does not own this file"})
				c.Abort()

				return
			}
		}
		log.Printf("[Middleware-Ownership] user: %v cleared ownership of resource: %v", ac.UID, ac.Target)
	}
}

/* This middleware should precheck if a user can claim access according
*  to the destined mode() on a resource
*
* mode should be read/write/execute
 */
func hasAccessMiddleware(mode string, srv *UService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// get header
		acH, exists := c.Get("accessTarget")
		if !exists {
			log.Printf("[Middleware-Access] failed to bind access-target header")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "access target header was not set correctly"})
			c.Abort()

			return
		}
		ac, ok := acH.(ut.AccessClaim)
		if !ok {
			log.Printf("[Middleware-Access] failed to cast ac, :?corrupt data?, should have been set already")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "acces-target setup error"})
			c.Abort()

			return
		}

		// root user has access to all
		if ac.UID == "0" {
			log.Printf("[Middleware-Access] root access detected, free pass...")
			c.Next()

			return
		}

		resources, err := accessTargets(c.Request.Context(), srv, ac)
		if err != nil {
			abortTargetError(c, "Middleware-Access", err)

			return
		}
		for _, resource := range resources {
			var allowed bool
			switch mode {
			case "r":
				allowed = resource.HasAccess(ac)
			case "w":
				allowed = resource.HasWriteAccess(ac)
			case "x":
				allowed = resource.HasExecutionAccess(ac)
			default:
				log.Printf("[Middleware-Access] unknown access mode %q", mode)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "bad setup"})
				c.Abort()

				return
			}
			if !allowed {
				log.Printf("[Middleware-Access] user %v has no %q access on %v", ac.UID, mode, resource.Name)
				c.JSON(http.StatusForbidden, gin.H{"error": "not allowed " + mode + " access on resource"})
				c.Abort()

				return
			}
		}
		log.Printf("[Middleware-Access] user: %v cleared access for resource: %v", ac.UID, ac.Target)
	}
}
