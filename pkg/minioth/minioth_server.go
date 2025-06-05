// Package minioth description
// @title           Minioth Auth API
// @version         1.0
// @description     API for user authentication and management using JWT.
// @host            minioth.local:9090
// @BasePath        /v1
// @schemes         http
package minioth

/* Minioth server is responsible for listening */

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	// swagger documentation
	_ "kyri56xcaesar/kuspace/api/minioth"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

/*
*
* Constants */
const (
	version = "v1"
)

var (
	mu               = sync.Mutex{}
	logMaxFetch      = 1000
	auditLogMaxFetch = 1000
)

// MService as a struct to encompass data for this server
/*
*
* minioth server central object.
* -> reference to a server engine
* -> configuration
* -> reference to minioth
*
*
* the model structs used by the server are defined in minioth.go
* */
type MService struct {
	Engine  *gin.Engine
	Minioth *Minioth
}

// NewMService function as a constructor for MService
// * "constructor of minioth server central object" */
func NewMSerivce(m *Minioth) MService {
	setGinMode(m.Config.APIGinMode)
	srv := MService{
		Minioth: m,
		Engine:  gin.Default(),
	}
	jwtSecretKey = srv.Minioth.Config.JwtSecretKey
	jwksFilePath = srv.Minioth.Config.Jwks
	jwtValidityHours = srv.Minioth.Config.JwtValidityHours
	issuer = srv.Minioth.Config.Issuer

	auditLogMaxFetch = srv.Minioth.Config.MiniothAuditLogsMaxFetch
	logMaxFetch = srv.Minioth.Config.APILogsMaxFetch
	_, err := os.Stat(auditLogPath)
	if err != nil {
		p := strings.Split(auditLogPath, "/")
		if len(p) < 2 {
			log.Fatalf("bad audit logs path")
		}
		err = os.MkdirAll(strings.Join(p[:len(p)-1], "/"), 0o644)
		if err != nil {
			log.Fatalf("couldn't make path directory: %v", err)
		}
		f, err := os.Create(auditLogPath)
		if err != nil {
			log.Fatalf("failed to touch the audit log file")
		}
		_, err = f.WriteString("==> minioth - audit logs <==\n")
		if err != nil {
			log.Printf("failed to write audit log header string...: %v", err)
		}
		defer func() {
			err := f.Close()
			if err != nil {
				log.Printf("failed to close file: %v", err)
			}
		}()
	}

	log.Printf("[INIT]updating jwt key...: %s", jwtSecretKey)
	log.Printf("[INIT]updating jwks path...: %s", jwksFilePath)
	log.Printf("[INIT]setting issuer to: %s", issuer)
	log.Printf("[INIT]setting audit max fetch...: %d", auditLogMaxFetch)
	log.Printf("[INIT]setting log max fetch...: %d", logMaxFetch)

	rotateKey()
	log.Printf("[INIT]rotating to new key: %v", signingKeys[currentKID])

	return srv
}

// ServeHTTP will launch the server listener
/* Should implement the following endpoints:
 * /login,  /register, /user/token, /token/refresh,
 * /groups, /groups/{groupID}/assign/{userID}
 * /token/refresh
 * /audit/logs, /admin/users, /admin/users
 */
func (srv *MService) ServeHTTP() {

	srv.RegisterRoutes()

	server := &http.Server{
		Addr:              srv.Minioth.Config.Addr(srv.Minioth.Config.APIPort),
		Handler:           srv.Engine,
		ReadHeaderTimeout: time.Second * 5,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if srv.Minioth.Config.APIUseTLS {
			if err := server.ListenAndServeTLS(srv.Minioth.Config.APICertFile, srv.Minioth.Config.APIKeyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %s\n", err)
			}
		} else {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %s\n", err)
			}
		}

	}()
	<-ctx.Done()

	log.Print("closing db connection...")
	srv.Minioth.handler.Close()

	stop()
	log.Println("shutting down gracefully, press Ctrl+C again to force")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}

// RegisterRoutes method will simply attach the endpoints to the server
func (srv *MService) RegisterRoutes() {
	apiV1 := srv.Engine.Group("/" + version)
	apiV1.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.InstanceName("miniothdocs")))
	{
		apiV1.POST("/register", srv.handleRegister)
		apiV1.POST("/login", srv.handleLogin)
		apiV1.POST("/passwd", AuthMiddleware("admin,user", srv), srv.handlePasswd)
		apiV1.GET("/user/me", srv.handleTokenUserInfo)
		apiV1.GET("/user/token", handleTokenInfo)
		apiV1.GET("/user/refresh-token", handleTokenRefresh)
	}

	/* admin endpoints */
	admin := apiV1.Group("/admin")
	if strings.ToLower(srv.Minioth.Config.APIGinMode) != "debug" {
		admin.Use(AuthMiddleware("admin", srv))
	}
	{
		admin.GET("/audit/logs", handleAuditLogs)
		admin.POST("/hasher", handleHasher)

		admin.POST("/verify-password", srv.handleVerifyPassword)
		admin.GET("/users", srv.handleUsers)
		admin.GET("/groups", srv.handleGroups)
		admin.POST("/useradd", srv.handleUseradd)
		admin.DELETE("/userdel", srv.handleUserdel)
		admin.PATCH("/userpatch", srv.handleUserpatch)
		admin.PUT("/usermod", srv.handleUsermod)
		admin.POST("/groupadd", srv.handleGroupadd)
		admin.PATCH("/grouppatch", srv.handleGrouppatch)
		admin.PUT("/groupmod", srv.handleGroupmod)
		admin.DELETE("/groupdel", srv.handleGroupdel)

		admin.POST("/rotate", func(c *gin.Context) {
			rotateKey()
			c.JSON(http.StatusOK, gin.H{"message": "key rotated"})
		})

		admin.Match(
			[]string{"GET"},
			"/system-conf",
			srv.handleSysConf,
		)
	}

	// these endpoints are not fully functional yet since our sign method is HS256 (no key needed)
	// TODO: yet (provide "identity" openid standard)
	wellknown := apiV1.Group("/.well-known")
	{
		wellknown.GET("/minioth", health)
		wellknown.GET("/openid-configuration", srv.openidConfiguration)
		wellknown.GET("/jwks.json", jwksHandler)
	}
}

/* Filter incoming login and register requests. Don't allow wierd chars...*/
func (l *LoginClaim) validateClaim() error {
	if l.Username == "" {
		return errors.New("username cannot be empty")
	}

	if !ut.IsAlphanumeric(l.Username) {
		return fmt.Errorf("username %q is invalid: only alphanumeric chararctes are allowed", l.Username)
	}

	if l.Password == "" {
		return errors.New("password cannot be empty")
	}

	return nil
}

func (u *RegisterClaim) validateUser() error {
	if u.User.Username == "" {
		return errors.New("username cannot be empty")
	}

	if len(u.User.Username) < 3 || len(u.User.Username) > 32 {
		return fmt.Errorf("username must be between 3 and 32 characters")
	}

	if offLimits(u.User.Username) {
		return errors.New("username off limits")
	}

	if !ut.IsAlphanumeric(u.User.Username) {
		return fmt.Errorf("username %q is invalid: only alphanumeric characters are allowed", u.User.Username)
	}

	if len(u.User.Info) > 100 {
		return fmt.Errorf("info field is too long: maximum allowed length is 100 characters")
	}

	if !ut.IsAlphanumericPlusSome(u.User.Info) {
		return fmt.Errorf("email %q is invalid: only alphanumeric characters[+@_] are allowed", u.User.Info)
	}

	if err := u.User.Password.ValidatePassword(); err != nil {
		return fmt.Errorf("password validation error: %w", err)
	}

	u.User.Username = strings.TrimSpace(u.User.Username)
	u.User.Info = strings.TrimSpace(u.User.Info)

	return nil
}

/* functions */
/* just a function to see if a given name is in input */
func offLimits(str string) bool {
	for _, name := range forbiddenNames {
		if str == name {
			return true
		}
	}
	return false
}

/* RegisterClaim struct covers Regist and Login requests binds */
type RegisterClaim struct {
	User ut.User `json:"user"`
}

// LoginCliam struct covers Login requests
type LoginClaim struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

/* JWT token signed claims.
* what information the jwt will contain.
* */

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
