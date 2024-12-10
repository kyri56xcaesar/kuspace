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

/*
*
* Constants */
const (
	apiPathPrefix string = "v1"
	templatesPath string = "./web/auther/templates"
	staticsPath   string = "web/auther/static"
)

/*
*
* Structs */
type HTTPService struct {
	Engine *gin.Engine
	Config *EnvConfig
}

/*
*
* Structs */
func NewService(conf string) HTTPService {
	service := HTTPService{}
	service.Engine = gin.Default()
	service.Config = LoadConfig(conf)
	log.Print(service.Config.ToString())

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
	srv.Engine.Use(cors.New(corsconfig))

	root := srv.Engine.Group("/")
	{
		root.GET("/", func(c *gin.Context) {
			log.Printf("%v request at %v from \n%v", c.Request.Method, c.Request.URL, c.Request.UserAgent())
			c.Redirect(http.StatusFound, "/api/"+apiPathPrefix)
		})

		root.GET("/healthz", func(c *gin.Context) {
			log.Printf("%v request at %v from \n%v", c.Request.Method, c.Request.URL, c.Request.UserAgent())
			c.JSON(http.StatusOK, gin.H{
				"status": "alive",
			})
		})
	}

	apiV1 := srv.Engine.Group("/api/" + apiPathPrefix)
	{
		apiV1.GET("/", func(c *gin.Context) {
			log.Printf("%v request at %v from \n%v", c.Request.Method, c.Request.URL, c.Request.UserAgent())
			c.HTML(http.StatusOK, "index.html", c.Request.UserAgent())
		})
		apiV1.GET("/login", func(c *gin.Context) {
			log.Printf("%v request at %v from \n%v", c.Request.Method, c.Request.URL, c.Request.UserAgent())
			c.HTML(http.StatusOK, "login.html", nil)
		})
		apiV1.POST("/login", srv.handleLogin)

		apiV1.GET("/register", func(c *gin.Context) {
			log.Printf("%v request at %v from \n%v", c.Request.Method, c.Request.URL, c.Request.UserAgent())
			c.HTML(http.StatusOK, "register.html", nil)
		})
		apiV1.POST("/register", srv.handleRegister)

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
			log.Printf("%v request at %v from \n%v", c.Request.Method, c.Request.URL, c.Request.UserAgent())

			username, err := c.Cookie("username")
			// NOTE: perhaps timeout or blacklist a malicious spammer...
			if err != nil {
				// Handle missing or invalid cookie
				log.Printf("missing username from cookie: %v", err)
				c.Redirect(http.StatusSeeOther, "/api/v1/login")
				return
			}

			accessToken, err := c.Cookie("access_token")
			if err != nil {
				log.Printf("missing access token: %v", err)
				c.Redirect(http.StatusSeeOther, "/api/v1/login")
				return
			}

			// TODO: : Decode and verify the token (e.g., JWT validation)
			// print it for now
			log.Print(accessToken)

			c.HTML(http.StatusOK, "admin-panel.html", gin.H{
				"username": username,
				"message":  "Welcome to the admin panel",
			})
		})
		verified.GET("/dashboard", AuthMiddleware(), func(c *gin.Context) {
			log.Printf("%v request at %v from \n%v", c.Request.Method, c.Request.URL, c.Request.UserAgent())

			username, err := c.Cookie("username")
			// NOTE: perhaps timeout or blacklist a malicious spammer...
			if err != nil {
				// Handle missing or invalid cookie
				log.Printf("unauthorized: %v", err)
				c.Redirect(http.StatusSeeOther, "/api/v1/login")
				return
			}

			accessToken, err := c.Cookie("access_token")
			if err != nil {
				log.Printf("missing access token: %v", err)
				c.Redirect(http.StatusSeeOther, "/api/v1/login")
				return
			}

			// TODO: : Decode and verify the token (e.g., JWT validation)
			// print it for now
			log.Print(accessToken)

			c.HTML(http.StatusOK, "dashboard.html", gin.H{
				"username": username,
				"message":  "your dashboard brother",
			})
		})
	}

	srv.Engine.NoRoute(func(c *gin.Context) {
		log.Printf("%v request at %v from \n%v", c.Request.Method, c.Request.URL, c.Request.UserAgent())
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
