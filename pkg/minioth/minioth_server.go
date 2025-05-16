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

	_ "kyri56xcaesar/kuspace/api/minioth"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

/*
*
* Constants */
const (
	VERSION = "v1"
)

var (
	mu                  = sync.Mutex{}
	log_max_fetch       = 1000
	audit_log_max_fetch = 1000
)

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

/*
*
* "constructor of minioth server central object" */
func NewMSerivce(m *Minioth) MService {
	setGinMode(m.Config.API_GIN_MODE)
	srv := MService{
		Minioth: m,
		Engine:  gin.Default(),
	}
	jwtSecretKey = srv.Minioth.Config.JWT_SECRET_KEY
	jwksFilePath = srv.Minioth.Config.JWKS
	jwtValidityHours = srv.Minioth.Config.JWT_VALIDITY_HOURS
	issuer = srv.Minioth.Config.ISSUER

	audit_log_max_fetch = srv.Minioth.Config.MINIOTH_AUDIT_LOGS_MAX_FETCH
	log_max_fetch = srv.Minioth.Config.API_LOGS_MAX_FETCH
	_, err := os.Stat(audit_log_path)
	if err != nil {
		p := strings.Split(audit_log_path, "/")
		if len(p) < 2 {
			log.Fatalf("bad audit logs path")
		}
		err = os.MkdirAll(strings.Join(p[:len(p)-1], "/"), 0o644)
		if err != nil {
			log.Fatalf("couldn't make path directory: %v", err)
		}
		f, err := os.Create(audit_log_path)
		if err != nil {
			log.Fatalf("failed to touch the audit log file")
		}
		f.WriteString("==> minioth - audit logs <==\n")
		defer f.Close()
	}

	log.Printf("[INIT]updating jwt key...: %s", jwtSecretKey)
	log.Printf("[INIT]updating jwks path...: %s", jwksFilePath)
	log.Printf("[INIT]setting issuer to: %s", issuer)
	log.Printf("[INIT]setting audit max fetch...: %d", audit_log_max_fetch)
	log.Printf("[INIT]setting log max fetch...: %d", log_max_fetch)

	rotateKey()
	log.Printf("[INIT]rotating to new key: %v", signingKeys[currentKID])

	return srv
}

/* Should implement the following endpoints:
 * /login,  /register, /user/token, /token/refresh,
 * /groups, /groups/{groupID}/assign/{userID}
 * /token/refresh
 * /audit/logs, /admin/users, /admin/users
 */
func (srv *MService) ServeHTTP() {

	srv.RegisterRoutes()

	server := &http.Server{
		Addr:              srv.Minioth.Config.Addr(srv.Minioth.Config.API_PORT),
		Handler:           srv.Engine,
		ReadHeaderTimeout: time.Second * 5,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if srv.Minioth.Config.API_USE_TLS {
			if err := server.ListenAndServeTLS(srv.Minioth.Config.API_CERT_FILE, srv.Minioth.Config.API_KEY_FILE); err != nil && err != http.ErrServerClosed {
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

func (srv *MService) RegisterRoutes() {
	apiV1 := srv.Engine.Group("/" + VERSION)
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
	if strings.ToLower(srv.Minioth.Config.API_GIN_MODE) != "debug" {
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
	}

	// these endpoints are not fully functional yet since our sign method is HS256 (no key needed)
	// TODO: yet (provide "identity" openid standard)
	wellknown := apiV1.Group("/.well-known")
	{
		wellknown.GET("/minioth", health)
		wellknown.GET("/openid-configuration", srv.openid_configuration)
		wellknown.GET("/jwks.json", jwks_handler)
	}
}

/* Filter incoming login and register requests. Don't allow wierd chars...*/
func (l *LoginClaim) validateClaim() error {
	if l.Username == "" {
		return errors.New("username cannot be empty")
	}

	if !ut.IsAlphanumericPlus(l.Username) {
		return fmt.Errorf("username %q is invalid: only alphanumeric chararctes[@+] are allowed", l.Username)
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

	if offLimits(u.User.Username) {
		return errors.New("username off limits")
	}

	if !ut.IsAlphanumericPlus(u.User.Username) {
		return fmt.Errorf("username %q is invalid: only alphanumeric characters[@+] are allowed", u.User.Username)
	}

	if len(u.User.Info) > 100 {
		return fmt.Errorf("info field is too long: maximum allowed length is 100 characters")
	}

	// Validate UID
	if u.User.Uid < 0 {
		return fmt.Errorf("uid '%d' is invalid: must be a non-negative integer", u.User.Uid)
	}

	// Validate Primary Group
	if u.User.Pgroup < 0 {
		return fmt.Errorf("primary group '%d' is invalid: must be a non-negative integer", u.User.Pgroup)
	}

	if err := u.User.Password.ValidatePassword(); err != nil {
		return fmt.Errorf("password validation error: %w", err)
	}

	return nil
}

/* functions */
/* just a function to see if a given name is in input */
func offLimits(str string) bool {
	for _, name := range forbidden_names {
		if str == name {
			return true
		}
	}
	return false
}

/* Incoming Register and Login requests binding structs */
type RegisterClaim struct {
	User ut.User `json:"user"`
}

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
