package userspace

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Api call Handlers
func (srv *UService) HandleJob(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
	case http.MethodPost:
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "method not allowed",
		})
	}
}

func (srv *UService) HandleJobAdmin(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
	case http.MethodPost:
	case http.MethodPut:
	case http.MethodDelete:
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "method not allowed",
		})
	}
}
