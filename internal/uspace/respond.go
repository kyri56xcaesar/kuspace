package uspace

import (
	"log"
	"net/http"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

// respondErr answers a failed operation with the status its error's kind
// maps to (ut.HTTPStatus). Client-side problems (4xx) carry the error's
// message; server-side ones are logged and answered with a generic one, so
// internals don't leak.
func respondErr(c *gin.Context, what string, err error) {
	status := ut.HTTPStatus(err)
	if status >= http.StatusInternalServerError && status != http.StatusInsufficientStorage {
		log.Printf("[USPACE] %s: %v", what, err)
		msg := what + " failed"
		if status == http.StatusServiceUnavailable {
			msg = what + ": storage is unavailable, try again later"
		}
		c.JSON(status, gin.H{"error": msg})

		return
	}
	c.JSON(status, gin.H{"error": err.Error()})
}
