package auther

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const (
	apiPathPrefix string = "v1"
)

type HTTPService struct {
	Engine *gin.Engine
	Config AConfig
}

func NewService() HTTPService {
	service := HTTPService{}
	service.Engine = gin.Default()
	service.Config = NewConfig()
	err := service.Config.LoadConfig("")
	if err != nil {
		log.Print(err)
	}
	log.Print(service.Config.toString())

	return service
}

func globalAPIRoutes(r *gin.RouterGroup, middleware ...gin.HandlerFunc) {
	r.Use(middleware...)
	// /ping
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "pong",
		})
	})

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	})
}

func adminAPIRoutes(r *gin.RouterGroup, middleware ...gin.HandlerFunc) {
	r.Use(middleware...)

	r.GET("", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"admin": "active",
		})
	})
}

func AuthMiddleWare() gin.HandlerFunc {
	return func(c *gin.Context) {
		// authentication
		// Here can Check for authentication from a different service
		// for now use this
		username := c.PostForm("username")
		password := c.PostForm("password")

		log.Printf("%v, %v", username, password)

		if username == "foo" {
			c.AbortWithStatus(http.StatusUnauthorized)
		}
	}
}

func (srv *HTTPService) MetaMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// c.Header("X-Whom", nil)
		c.Next()
	}
}

func (srv *HTTPService) ServeHTTP() {
	corsconfig := cors.DefaultConfig()
	corsconfig.AllowOrigins = srv.Config.AllowedOrigins
	corsconfig.AllowMethods = srv.Config.AllowedMethods
	corsconfig.AllowHeaders = srv.Config.AllowedHeaders
	corsconfig.ExposeHeaders = srv.Config.ExposeHeaders

	// General
	rootPath := srv.Engine.Group("/")
	rootPath.Use(cors.New(corsconfig))
	rootPath.Use(srv.MetaMiddleware())

	globalAPIRoutes(rootPath)

	// Auth Group
	authenticated := rootPath.Group("/admin", gin.BasicAuth(gin.Accounts{
		"foo": "bar",
	}))

	adminAPIRoutes(authenticated, AuthMiddleWare())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr:              srv.Config.Addr(),
		Handler:           srv.Engine,
		ReadHeaderTimeout: time.Second * 5,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-ctx.Done()

	stop()
	log.Println("shutting down gracefully, press Ctrl+C again to force")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}
