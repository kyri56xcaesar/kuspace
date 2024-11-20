package auther

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"strings"
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

	// Setup Security Headers
	//srv.Engine.Use(func(c *gin.Context) {
	//	if c.Request.Host != srv.Config.Addr() {
	//		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid host header"})
	//		return
	//	}
	//	c.Header("X-Frame-Options", "DENY")
	//	c.Header("Content-Security-Policy", "default-src 'self'; connect-src *; font-src *; script-src-elem * 'unsafe-inline'; img-src * data:; style-src * 'unsafe-inline';")
	//	c.Header("X-XSS-Protection", "1; mode=block")
	//	c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
	//	c.Header("Referrer-Policy", "strict-origin")
	//	c.Header("X-Content-Type-Options", "nosniff")
	//	c.Header("Permissions-Policy", "geolocation=(),midi=(),sync-xhr=(),microphone=(),camera=(),magnetometer=(),gyroscope=(),fullscreen=(self),payment=()")
	//	c.Next()
	//})

	// General
	srv.Engine.Use(cors.New(corsconfig))
	srv.Engine.Use(srv.MetaMiddleware())

	// Api
	apiV1 := srv.Engine.Group("/api/" + apiPathPrefix)
	{
		apiV1.Static("/login", "./cmd/auther/static")
	}

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

func isBrowser(userAgent string) bool {
	browsers := []string{"Mozilla", "Chrome", "Safari", "Edge", "Opera"}
	for _, browser := range browsers {
		if strings.Contains(userAgent, browser) {
			return true
		}
	}
	return false
}
