package fslite

import (
	"context"
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
	ut "kyri56xcaesar/kuspace/internal/utils"
)

const (
	version string = "/"
)

// ListenAndServe starts the FsLite HTTP server with all configured routes and middleware.
// It sets up health check, authentication, admin, and resource management endpoints.
// The server listens for system interrupt signals to gracefully shut down, closing
// database connections and waiting for in-flight requests to complete before exiting.
func (fsl *FsLite) ListenAndServe() {
	srv := fsl.engine

	srv.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})
	api := srv.Group(version)
	{
		api.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.InstanceName("fslitedocs")))

		api.POST("/login", fsl.loginHandler)
	}

	admin := api.Group("/admin")
	// have authentication only on release
	if strings.ToLower(fsl.config.APIGinMode) != "debug" {
		admin.Use(authmiddleware(fsl.config))
	}
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

func authmiddleware(cfg ut.EnvConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if sSecretClaim := c.GetHeader("X-Service-Secret"); sSecretClaim != "" {
			if sSecretClaim == string(cfg.ServiceSecretKey) {
				log.Printf("[FSL_SERVER_middleware] service secret accepted. access granted.")
				c.Next()

				return
			}
			log.Printf("[FSL_SERVER_middleware] service secret invalid. access not granted")
			c.Abort()

			return
		}
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			log.Printf("[FSL_SERVER_middleware] authorization header not found")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()

			return
		}

		// Extract the token from the Authorization header
		tokenString := authHeader[len("Bearer "):]
		if tokenString == "" {
			log.Printf("[FSL_SERVER_middleware] bearer token not found")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token is required"})
			c.Abort()

			return
		}

		ok, claims, err := decodeJWT(tokenString)
		if err != nil {
			log.Printf("[FSL_SERVER_middleware] token bad format")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode token"})
			c.Abort()

			return
		}

		// Set claims in the context for further use
		if ok {
			c.Set("username", claims.Username)
			c.Set("uid", claims.ID)
		} else {
			log.Printf("[FSL_SERVER_middleware] invalid token claims")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()

			return
		}

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
