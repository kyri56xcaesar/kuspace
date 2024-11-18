package auther

import (
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const (
	apiPathPrefix string = "v1"
)

type HTTPService struct {
	Engine *gin.Engine
}

func (srv *HTTPService) ServeHTTP() {
	srv.Engine = gin.Default()

	rootPath := srv.Engine.Group("/")

	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"*"}
	config.AllowHeaders = []string{"*"}
	config.ExposeHeaders = []string{"*"}

	rootPath.Use(cors.New(config))
	rootPath.Use(srv.MetaMiddleware())

	srv.APIRoutes(rootPath)

	// go func() {
	if err := srv.Engine.Run(); err != nil {
		log.Printf("%v", err)
	}
	//}()
}

func (srv *HTTPService) APIRoutes(r *gin.RouterGroup, middleware ...gin.HandlerFunc) {
	srv.Engine.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "pong",
		})
	})
}

func (srv *HTTPService) MetaMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// c.Header("X-Whom", nil)
		c.Next()
	}
}
