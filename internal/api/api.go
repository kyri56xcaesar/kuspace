package api

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os/signal"
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

	// template functions
	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
	}

	// set a template eng
	tmpl := template.Must(template.New("").Funcs(funcMap).ParseGlob(templatesPath + "/*.html"))

	// Api
	srv.Engine.SetHTMLTemplate(tmpl)
	srv.Engine.Use(static.Serve("/api/"+apiPathPrefix, static.LocalFile(staticsPath, true)))
	srv.Engine.Use(cors.New(corsconfig))

	root := srv.Engine.Group("/")
	{
		root.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusFound, "/api/"+apiPathPrefix)
		})

		root.GET("/healthz", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "alive",
			})
		})
	}

	apiV1 := srv.Engine.Group("/api/" + apiPathPrefix)
	{
		apiV1.GET("/", func(c *gin.Context) {
			c.HTML(http.StatusOK, "index.html", c.Request.UserAgent())
		})
		apiV1.GET("/login", autoLogin(), func(c *gin.Context) {
			c.HTML(http.StatusOK, "login.html", nil)
		})
		apiV1.POST("/login", srv.handleLogin)

		apiV1.GET("/register", func(c *gin.Context) {
			c.HTML(http.StatusOK, "register.html", nil)
		})
		apiV1.POST("/register", srv.handleRegister)

		apiV1.DELETE("/logout", func(c *gin.Context) {
			params := c.Request.URL.Query()

			if params == nil {
				c.JSON(http.StatusBadRequest, gin.H{"status": "no params specified"})
				return
			}

			for key := range params {
				// essentially overwrites and eventually gets the cookie deleted.
				c.SetCookie(key, "", 1, "/api/v1/", "", false, true) // Set the username cookie
			}
			log.Print("cookies deleted")
			c.Redirect(300, "/api/v1/login")
		})

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
		verified.GET("/admin-panel", AuthMiddleware("admin"), func(c *gin.Context) {
			username, _ := c.Cookie("username")
			c.HTML(http.StatusOK, "admin-panel.html", gin.H{
				"username": username,
				"message":  "Welcome to the Admin Panel ",
			})
		})
		verified.GET("/admin/fetch-users", srv.handleFetchUsers)
		verified.GET("/admin/fetch-groups", srv.handleFetchGroups)
		verified.POST("/admin/useradd", srv.handleUseradd)
		verified.DELETE("/admin/userdel", srv.handleUserdel)
		verified.PATCH("/admin/userpatch", srv.handleUserpatch)
		verified.POST("/admin/groupadd", srv.handleGroupadd)
		verified.DELETE("/admin/groupdel", srv.handleGroupdel)
		verified.PATCH("/admin/grouppatch", srv.handleGrouppatch)

		verified.GET("/admin/hasher", func(c *gin.Context) {
			c.HTML(http.StatusOK, "hasher_template.html", nil)
		})
		verified.POST("/admin/hasher", srv.handleHasher)

		verified.GET("/dashboard", AuthMiddleware("user, admin"), func(c *gin.Context) {
			username, _ := c.Cookie("username")
			c.HTML(http.StatusOK, "dashboard.html", gin.H{
				"username": username,
				"message":  "your dashboard ",
			})
		})
	}

	srv.Engine.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"error":   "404",
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
