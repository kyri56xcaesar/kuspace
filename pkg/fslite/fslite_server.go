package fslite

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// fslite swagger docs
	_ "kyri56xcaesar/kuspace/api/fslite"
)

const (
	version string = "/"
)

// ListenAndServe starts the FsLite HTTP server with all configured routes and middleware.
// It sets up health check, authentication, admin, and resource management endpoints.
// The server listens for system interrupt signals to gracefully shut down, closing
// database connections and waiting for in-flight requests to complete before exiting.
func (fsl *FsLite) ListenAndServe() {
	fsl.serve(fsl.routes())
}

// routes registers the API on fsl.Engine and returns it.
func (fsl *FsLite) routes() *gin.Engine {
	srv := fsl.Engine

	srv.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})
	api := srv.Group(version)
	{
		api.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.InstanceName("fslitedocs")))

		api.POST("/login", fsl.loginHandler)
	}

	admin := api.Group("/admin")
	// authentication is unconditional (it used to be skipped in gin "debug" mode)
	admin.Use(fsl.authmiddleware())
	{
		admin.POST("/register", fsl.registerHandler)

		admin.POST("/volume/new", fsl.newVolumeHandler)
		admin.DELETE("/volume/delete", fsl.deleteVolumeHandler)
		admin.GET("/volume/get", fsl.getVolumeHandler)

		admin.GET("/resource/get", fsl.getResourceHandler)
		admin.GET("/resource/stat", fsl.statResourceHandler)
		admin.DELETE("/resource/delete", fsl.deleteResourceHandler)
		admin.POST("/resource/copy", fsl.copyResourceHandler)

		admin.POST("/resource/upload", fsl.uploadResourceHandler)
		admin.GET("/resource/download", fsl.downloadResourceHandler)

		// admin.GET("/resource/share", fsl.shareResourceHandler)

		admin.Match([]string{"GET", "PATCH", "DELETE"}, "/user/volumes", fsl.handleUserVolumes)
		admin.Match([]string{"GET"}, "/system-conf", fsl.handleSysConf)
	}

	return srv
}

// serve runs the server until SIGINT/SIGTERM, then shuts down gracefully.
func (fsl *FsLite) serve(srv *gin.Engine) {
	server := &http.Server{
		Addr:              fsl.config.Addr(fsl.config.APIPort),
		Handler:           srv,
		ReadHeaderTimeout: time.Second * 5,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[FSL_SERVER] listen: %s\n", err)
		}
	}()
	<-ctx.Done()

	log.Print("[FSL_SERVER] closing db connection...")
	fsl.dbh.Close()

	stop()
	log.Println("[FSL_SERVER] shutting down gracefully, press Ctrl+C again to force")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("[FSL_SERVER] Server forced to shutdown: ", err)
	}

	log.Println("[FSL_SERVER] Server exiting")
}

// authmiddleware admits services (X-Service-Secret) and admins with a valid
// fslite token, both acting as uid 0; everyone else gets 401.
func (fsl *FsLite) authmiddleware() gin.HandlerFunc {
	cfg := fsl.config

	return func(c *gin.Context) {
		if claim := c.GetHeader("X-Service-Secret"); claim != "" {
			if len(cfg.ServiceSecretKey) > 0 && subtle.ConstantTimeCompare([]byte(claim), cfg.ServiceSecretKey) == 1 {
				c.Set("uid", "0")
				c.Next()

				return
			}
			// used to Abort without a status: an empty 200
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid service secret"})

			return
		}
		// used to slice the header blindly, panicking on short values
		tokenString, found := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !found || tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "a bearer token is required"})

			return
		}

		claims, err := fsl.tokens.verify(tokenString)
		if err != nil {
			log.Printf("[FSL_SERVER_middleware] rejected token: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})

			return
		}
		c.Set("username", claims.Username)
		c.Set("admin_id", claims.ID)
		// admins are the store's superusers: they act as root (uid 0).
		// uid used to be the admin's UUID, which the upload and delete
		// handlers can't parse: every admin upload failed with 500.
		c.Set("uid", "0")

		c.Next()
	}
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
