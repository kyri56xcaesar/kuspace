package userspace

import "github.com/gin-gonic/gin"

/* 'resource' handlers
* SEE: models.go
* */
func (srv *UService) GetFile(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "fs",
	})
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
