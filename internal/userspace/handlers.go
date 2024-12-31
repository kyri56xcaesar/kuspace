package userspace

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

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
func BindAccessTarget(http_header string) (*AccessClaim, error) {
	log.Printf("trying to bind header: %s", http_header)

	parts := strings.SplitN(http_header, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid header format")
	}

	target := parts[0]
	ftype := "file"
	if strings.HasSuffix(parts[0], "/") {
		ftype = "dir"
		target, _ = strings.CutSuffix(target, "/")
	}

	sig := parts[1]
	p := strings.SplitN(sig, ":", 2)
	if len(p) != 2 {
		return nil, fmt.Errorf("invalid signature format")
	}

	return &AccessClaim{
		Uid:    p[0],
		Gids:   p[1],
		Target: target,
		Type:   ftype,
	}, nil
}

/* 'resource' handlers
* SEE: models.go
* */
func (srv *UService) GetFile(c *gin.Context) {
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil {
		log.Printf("failed to bind access-target: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
		return
	}
	log.Printf("binded access claim: %+v", ac)
	/* check if claim is valid */
	/* It is checked on binding rn
	  if err := ac.validate(); err != nil {
			log.Printf("claim not valid: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	*/

	/* find target resource */
	resource, err := srv.dbh.GetResourceByFilename(ac.Target)
	if err != nil {
		log.Printf("error retrieving resource: %v", err)
		if strings.Contains(err.Error(), "scan") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "fatal"})
		}
		return
	}

	/* Check for access authorization */
	/* This method requires Read Access to the Resource */
	if !resource.HasAccess(*ac) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	c.JSON(200, resource)
}

func (srv *UService) GetFiles(c *gin.Context) {
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil {
		log.Printf("failed to bind access-target: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
		return
	}
	log.Printf("binded access claim: %+v", ac)

	resources, err := srv.dbh.GetAllResources()
	if err != nil {
		log.Printf("error retrieving resource: %v", err)
		if strings.Contains(err.Error(), "scan") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "fatal"})
		}
		return
	}

	c.JSON(200, resources)
}

/* */
func (srv *UService) PostFile(c *gin.Context) {
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil {
		log.Printf("failed to bind access-target: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
		return
	}
	log.Printf("binded access claim: %+v", ac)

	var resources []Resource
	err = c.BindJSON(&resources)
	if err != nil {
		log.Printf("error binding: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad binding"})
		return
	}
	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
	for i := range resources {
		resources[i].Created_at = currentTime
		resources[i].Updated_at = currentTime
		resources[i].Accessed_at = currentTime
	}

	log.Printf("binded resources: %+v", resources)

	err = srv.dbh.InsertResources(resources)
	if err != nil {
		log.Printf("failed to insert resources: %v", err)
		c.JSON(422, gin.H{"error": "failed to insert resources"})
		return
	}

	c.JSON(200, gin.H{
		"message": "resources inserted",
	})
}

func (srv *UService) PatchFile(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "fs",
	})
}

func (srv *UService) DeleteFiles(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "fs",
	})
}

/* Volume handlers */
func (srv *UService) GetVolumes(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "v",
	})
}

func (srv *UService) PostVolumes(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "vp",
	})
}

func (srv *UService) DeleteVolumes(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "vpp",
	})
}
