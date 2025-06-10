package uspace_test

import (
	"github.com/gin-gonic/gin"

	"kyri56xcaesar/kuspace/internal/uspace"
)

func setupTestServer() *gin.Engine {
	gin.SetMode(gin.TestMode)
	srv := uspace.NewUService("tests/uspace/uspace_test.conf")
	srv.RegisterRoutes()
	return srv.Engine
}
