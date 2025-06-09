// Package frontendapp defines the service of the frontend
package frontendapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

/*
*
* Constants */
const (
	apiVersion    = "v1"
	templatesPath = "./web/templates"
	staticsPath   = "web/static"
)

// HTTPService struct holds the engine and the configuration
/*
*
* Structs */
type HTTPService struct {
	Engine *gin.Engine
	Config ut.EnvConfig
}

// NewService function as in a constructor for HTTPService struct
/*
*
* Structs */
func NewService(conf string) HTTPService {
	service := HTTPService{}
	service.Config = ut.LoadConfig(conf)
	setGinMode(service.Config.APIGinMode)
	service.Engine = gin.Default()

	authServiceURL = fmt.Sprintf("http://%s", net.JoinHostPort(service.Config.AuthAddress, service.Config.AuthPort))
	apiServiceURL = fmt.Sprintf("http://%s", net.JoinHostPort(service.Config.APIAddress, service.Config.APIPort))
	if strings.ToLower(service.Config.Profile) == "container" {
		wssServiceURL = "http://" + service.Config.WssAddressInternal
	} else {
		wssServiceURL = "http://" + service.Config.WssAddress
	}

	return service
}

// ServeHTTP function launces the listening server
func (srv *HTTPService) ServeHTTP() {
	corsconfig := cors.DefaultConfig()
	corsconfig.AllowOrigins = srv.Config.AllowedOrigins
	corsconfig.AllowMethods = srv.Config.AllowedMethods
	corsconfig.AllowHeaders = srv.Config.AllowedHeaders

	// template functions
	funcMap := template.FuncMap{
		"add": func(a, b any) float64 {
			return ut.ToFloat64(a) + ut.ToFloat64(b)
		},
		"sub": func(a, b any) float64 {
			return ut.ToFloat64(a) - ut.ToFloat64(b)
		},
		"mul": func(a, b any) float64 {
			return ut.ToFloat64(a) * ut.ToFloat64(b)
		},
		"div": func(a, b any) float64 {
			if ut.ToFloat64(b) == 0 {
				return 0
			}

			return ut.ToFloat64(a) / ut.ToFloat64(b)
		},
		"typeIs": func(value any, t string) bool {
			return reflect.TypeOf(value).Kind().String() == t
		},
		"hasKey": func(value map[string]any, key string) bool {
			_, exists := value[key]

			return exists
		},
		"lt": func(a, b any) bool {
			return ut.ToFloat64(a) < ut.ToFloat64(b)
		},
		"gr": func(a, b any) bool {
			return ut.ToFloat64(a) > ut.ToFloat64(b)
		},
		"index": func(m map[int]any, key int) any {
			if val, ok := m[key]; ok {
				return val
			}

			return nil // Return nil if key does not exist
		},
		"findGroupVolume": func(s []ut.GroupVolume, gid int) *ut.GroupVolume {
			for _, v := range s {
				if v.GID == gid {
					return &v
				}
			}

			return nil
		},
		"findUserVolume": func(s []ut.UserVolume, uid int) *ut.UserVolume {
			for _, v := range s {
				if v.UID == uid {
					return &v
				}
			}

			return nil
		},
		"toJSON": func(v any) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("{}")
			}

			return template.JS(b)
		},
		"lower":     strings.ToLower,
		"bytesToMB": bytesToMB,
		"ago":       ago,
	}

	// set a template eng
	tmpl := template.Must(template.New("").Funcs(funcMap).ParseGlob(templatesPath + "/*.html"))

	// Api
	srv.Engine.SetHTMLTemplate(tmpl)
	srv.Engine.Use(static.Serve("/api/"+apiVersion, static.LocalFile(staticsPath, true)))
	srv.Engine.Use(cors.New(corsconfig))

	if srv.Config.APIGinMode == "release" {
		log.Printf("security middleware currently deactivated...")
		// srv.Engine.Use(securityMiddleWare)
	}

	root := srv.Engine.Group("/")
	{
		root.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusFound, "/api/"+apiVersion+"/login")
		})

		root.GET("/healthz", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "alive",
			})
		})

		root.GET("/conf", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"ws_address": srv.Config.WssAddress,
			})
		})
	}

	apiV1 := srv.Engine.Group("/api/" + apiVersion)
	{
		apiV1.GET("/", func(c *gin.Context) {
			c.HTML(http.StatusOK, "login.html", c.Request.UserAgent())
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
			c.Redirect(http.StatusMultipleChoices, "/api/v1/login")
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
	verified.Use(authMiddleware("user,admin"))
	{
		// actions
		verified.POST("/passwd", srv.passwordChangeHandler)
		verified.PUT("/user-update", srv.updateUser)
		verified.POST("/upload", srv.handleResourceUpload)
		verified.GET("/download", srv.handleResourceDownload)
		verified.GET("/preview", srv.handleResourcePreview)
		verified.PATCH("/mv", srv.handleResourceMove)
		verified.POST("/cp", srv.handleResourceCopy)
		verified.DELETE("/rm", srv.handleResourceDelete)
		// shell (experimental)
		verified.GET("/gshell", func(c *gin.Context) {
			whoami, exists := c.Get("username")
			if !exists {
				whoami = "jondoe"
			}
			c.HTML(http.StatusOK, "gshell-display.html", gin.H{"whoami": whoami})
		})

		// render page
		verified.GET("/admin-panel", srv.handleAdminPanel)
		verified.GET("/edit-form", srv.editFormHandler)
		// fetch data
		// jobs

		verified.POST("/jobs", srv.jobsHandler)
		verified.GET("/fetch-resources", srv.handleFetchResources) // we want to allow users as well
		verified.GET("/fetch-volumes", srv.handleFetchVolumes)
		verified.GET("/fetch-jobs", srv.jobsHandler)
		verified.GET("/fetch-apps", srv.appsHandler)

		admin := verified.Group("/admin")
		admin.PATCH("/chmod", authMiddleware("user,admin"), srv.handleResourcePerms)
		admin.Use(authMiddleware("admin"))
		/* minioth will verify token no need to worry here.*/
		{
			admin.Match(
				[]string{"GET"},
				"/system-conf",
				srv.handleSysConf,
			)

			admin.GET("/system-metrics", srv.handleSysMetrics)

			admin.Match(
				[]string{"GET", "POST", "PUT", "DELETE"},
				"/jobs",
				srv.jobAdminHandler,
			)
			admin.Match(
				[]string{"GET", "POST", "PUT", "DELETE"},
				"/apps",
				srv.appAdminHandler,
			)
			admin.GET("/fetch-users", srv.handleFetchUsers)
			admin.GET("/fetch-groups", srv.handleFetchGroups)

			admin.POST("/useradd", srv.handleUseradd)
			admin.DELETE("/userdel", srv.handleUserdel)
			admin.PATCH("/userpatch", srv.handleUserpatch)
			admin.POST("/groupadd", srv.handleGroupadd)
			admin.DELETE("/groupdel", srv.handleGroupdel)
			admin.PATCH("/grouppatch", srv.handleGrouppatch)
			admin.PATCH("/chown", srv.handleResourcePerms)
			admin.PATCH("/chgroup", srv.handleResourcePerms)

			admin.POST("/volumeadd", srv.handleVolumeadd)
			admin.DELETE("/volumedel", srv.handleVolumedel)

			admin.POST("/hasher", srv.handleHasher)
		}
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
		Addr:              srv.Config.Addr(srv.Config.FrontPort),
		Handler:           srv.Engine,
		ReadHeaderTimeout: time.Second * 5,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

func setGinMode(mode string) {
	switch strings.ToLower(mode) {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "debug":
		gin.SetMode(gin.DebugMode)
	case "envgin":
		gin.SetMode(gin.EnvGinMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}
}

func bytesToMB(bytes any) string {
	switch v := bytes.(type) {
	case int64:

		return fmt.Sprintf("%.1f", float64(v)/1024.0/1024.0)
	case float64:

		return fmt.Sprintf("%.1f", v/1024.0/1024.0)
	case string:
		if num, err := strconv.ParseInt(v, 10, 64); err == nil {
			return fmt.Sprintf("%.1f", float64(num)/1024.0/1024.0)
		}
	}

	return "N/A"
}

func ago(t any) string {
	var parsed time.Time
	switch v := t.(type) {
	case time.Time:
		parsed = v
	case string:
		var err error
		parsed, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return "invalid time"
		}
	default:

		return "unknown time"
	}

	diff := time.Since(parsed)
	switch {
	case diff < time.Minute:

		return "just now"
	case diff < time.Hour:

		return fmt.Sprintf("%d min ago", int(diff.Minutes()))
	case diff < 24*time.Hour:

		return fmt.Sprintf("%d hr ago", int(diff.Hours()))
	default:

		return fmt.Sprintf("%d days ago", int(diff.Hours()/24))
	}
}
