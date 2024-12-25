package userspace

import "github.com/gin-gonic/gin"

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
