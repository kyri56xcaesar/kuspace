package api

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

const (
	apiPathPrefix string = "v1"
	templatesPath string = "./web/auther/templates"
	staticsPath   string = "web/auther/static"
)

var jwtSecretKey = []byte("default_placeholder_key")

type HTTPService struct {
	Engine *gin.Engine
	Config *EnvConfig
}

func NewService(conf string) HTTPService {
	service := HTTPService{}
	service.Engine = gin.Default()
	service.Config = LoadConfig(conf)
	log.Print(service.Config.ToString())

	jwtSecretKey = service.Config.JWTSecretKey

	return service
}

// Serve function
func (srv *HTTPService) ServeHTTP() {
	corsconfig := cors.DefaultConfig()
	corsconfig.AllowOrigins = srv.Config.AllowedOrigins
	corsconfig.AllowMethods = srv.Config.AllowedMethods
	corsconfig.AllowHeaders = srv.Config.AllowedHeaders

	// Api
	srv.Engine.LoadHTMLGlob(templatesPath + "/*.html")
	srv.Engine.Use(static.Serve("/api/"+apiPathPrefix, static.LocalFile(staticsPath, true)))

	apiV1 := srv.Engine.Group("/api/" + apiPathPrefix)

	globalAPIRoutes(apiV1, securityMiddleWare, cors.New(corsconfig))

	{
		apiV1.GET("/", func(c *gin.Context) {
			c.HTML(http.StatusOK, "index.html", c.Request.UserAgent())
		})
		apiV1.GET("/login", func(c *gin.Context) {
			c.HTML(http.StatusOK, "login.html", nil)
		})
		apiV1.POST("/login", srv.handleLogin)

	}

	oauth := apiV1.Group("/auth")
	{
		oauth.GET("/google", googleAuthHandler)
		oauth.GET("/google/callback", googleCallbackHandler)
		oauth.GET("/github", githubAuthHandler)
		oauth.GET("/github/callback", githubCallbackHandler)
	}

	verified := apiV1.Group("/verified")
	{
		verified.GET("/admin-panel", AuthMiddleware(), func(c *gin.Context) {
			username, _ := c.Get("username")
			c.HTML(http.StatusOK, "admin-panel.html", gin.H{
				"username": username,
			})
		})
		verified.GET("/dashboard", AuthMiddleware(), func(c *gin.Context) {
			username, _ := c.Get("username")
			c.HTML(http.StatusOK, "dashboard.html", gin.H{
				"username": username,
			})
		})
	}

	srv.Engine.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"error":   strconv.Itoa(http.StatusNotFound),
			"message": "Not found",
		})
	})

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

// Global handlers
// global middleware
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

// Routing handlers
// Admin, admin middleware
func adminAPIRoutes(r *gin.RouterGroup, middleware ...gin.HandlerFunc) {
	r.Use(middleware...)

	r.GET("", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"admin": "active",
		})
	})
}
