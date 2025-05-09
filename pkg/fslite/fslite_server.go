package fslite

import (
	"context"
	ut "kyri56xcaesar/myThesis/internal/utils"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	version string = "/"
)

func (fsl *FsLite) ListenAndServe() {
	srv := fsl.engine

	srv.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})
	api := srv.Group(version)
	{
		api.POST("/login", fsl.loginHandler)
	}

	admin := api.Group("/admin")
	// have authentication only on release
	if strings.ToLower(fsl.config.API_GIN_MODE) != "debug" {
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
	}

	server := &http.Server{
		Addr:              fsl.config.Addr(fsl.config.API_PORT),
		Handler:           srv,
		ReadHeaderTimeout: time.Second * 5,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	<-ctx.Done()

	log.Print("closing db connection...")
	fsl.dbh.Close()

	stop()
	log.Println("shutting down gracefully, press Ctrl+C again to force")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}

func authmiddleware(cfg ut.EnvConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s_secret_claim := c.GetHeader("X-Service-Secret"); s_secret_claim != "" {
			if s_secret_claim == string(cfg.ServiceSecret) {
				log.Printf("service secret accepted. access granted.")
				c.Next()
				return
			} else {
				log.Printf("service secret invalid. access not granted")
				c.Abort()
				return
			}
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// Extract the token from the Authorization header
		tokenString := authHeader[len("Bearer "):]
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token is required"})
			c.Abort()
			return
		}

		ok, claims, err := decodeJWT(tokenString)
		if err != nil {
			log.Printf("failed to decode token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode token"})
			c.Abort()
			return
		}

		// Set claims in the context for further use
		if ok {
			c.Set("username", claims.Username)
			c.Set("uid", claims.ID)
		} else {
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
