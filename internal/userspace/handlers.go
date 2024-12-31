package userspace

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

/* 'resource' handlers
* SEE: models.go
* */
func (srv *UService) GetFile(c *gin.Context) {
	var ac AccessClaim
	err := c.BindJSON(&ac)
	if err != nil {
		log.Printf("failed to bind body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad binding"})
		return
	}

	/* check if claim is valid */
	if err := ac.validate(); err != nil {
		log.Printf("claim not valid: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
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
	if !resource.HasAccess(ac) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	c.JSON(200, resource)
}

func (srv *UService) GetFiles(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "fs",
	})
}

func (srv *UService) PostFile(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "fs",
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
