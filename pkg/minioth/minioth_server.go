// @title           Minioth Auth API
// @version         1.0
// @description     API for user authentication and management using JWT.
// @host            localhost:8080
// @BasePath        /v1
// @schemes         http
package minioth

/* Minioth server is responsible for listening */

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	ut "kyri56xcaesar/myThesis/internal/utils"

	_ "kyri56xcaesar/myThesis/api/minioth"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

/*
*
* Constants */
const (
	VERSION = "v1"
)

/*
*
* Variables */
var (
	audit_log_path     = "data/logs/minioth/audit.log"
	log_path           = "data/logs/minioth/minioth.log"
	jwtSecretKey       = []byte("default_placeholder_key")
	jwtRefreshKey      = []byte("default_refresh_placeholder_key")
	jwksFilePath       = "jwks.json"
	JWT_VALIDITY_HOURS = 1

	forbidden_names []string = []string{
		"root",
		"kubernetes",
		"k8s",
	}
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
	jwtSecretKey = srv.Minioth.Config.JWTSecretKey
	jwtRefreshKey = srv.Minioth.Config.JWTRefreshKey
	jwksFilePath = srv.Minioth.Config.JWKS
	audit_log_path = srv.Minioth.Config.MINIOTH_AUDIT_LOGS
	log_path = srv.Minioth.Config.MINIOTH_LOGS

	log.Printf("updating jwt key...: %s", jwtSecretKey)
	log.Printf("updating jwt refresh key...: %s", jwtRefreshKey)
	log.Printf("setting hashcost to: HASH_COST=%v", HASH_COST)
	log.Printf("setting audit log to: %s", audit_log_path)
	log.Printf("setting logs to: %s", log_path)

	return srv
}

/* Should implement the following endpoints:
 * /login,  /register, /user/token, /token/refresh,
 * /groups, /groups/{groupID}/assign/{userID}
 * /token/refresh
 * /audit/logs, /admin/users, /admin/users
 */
func (srv *MService) ServeHTTP() {
	apiV1 := srv.Engine.Group("/" + VERSION)
	apiV1.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	{
		apiV1.POST("/register", srv.handleRegister)
		apiV1.POST("/login", srv.handleLogin)
		apiV1.GET("/user/me", srv.handleTokenUserInfo)
		apiV1.POST("/passwd", srv.handlePasswd)
		apiV1.POST("/token/refresh", handleTokenRefresh)
		apiV1.GET("/user/token", handleTokenInfo)
	}

	/* admin endpoints */
	admin := apiV1.Group("/admin")
	admin.Use(AuthMiddleware("admin", srv))
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
	}

	// these endpoints are not fully functional yet since our sign method is HS256 (no key needed)
	// TODO: yet (provide "identity" openid standard)
	wellknown := apiV1.Group("/.well-known")
	{
		wellknown.GET("/minioth", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"version": "0.0.1",
				"status":  "alive",
			})
		})
		wellknown.GET("/openid-configuration", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"issuer":   srv.Minioth.Config.ISSUER,
				"jwks_uri": fmt.Sprintf("%s/.well-known/jwks.json", srv.Minioth.Config.ISSUER),
				// "authorization_endpoint":                fmt.Sprintf("%s/%s/login", srv.Config.ISSUER, VERSION),
				"token_endpoint":                        fmt.Sprintf("%s/%s/login", srv.Minioth.Config.ISSUER, VERSION),
				"userinfo_endpoint":                     fmt.Sprintf("%s/%s/user/me", srv.Minioth.Config.ISSUER, VERSION),
				"id_token_signing_alg_values_supported": "HS256",
			})
		})

		wellknown.GET("/jwks.json", func(c *gin.Context) {
			jwksFile, err := os.Open(jwksFilePath)
			if err != nil {
				log.Printf("failed to open jwks.json file: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load JWKS"})
				return
			}
			defer jwksFile.Close()

			jwksData, err := io.ReadAll(jwksFile)
			if err != nil {
				log.Printf("failed to read jwks.json file: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read JWKS"})
				return
			}

			var jwks map[string]any
			if err := json.Unmarshal(jwksData, &jwks); err != nil {
				log.Printf("failed to parse jwks.json file: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse JWKS"})
				return
			}

			c.JSON(http.StatusOK, jwks)
		})
	}

	server := &http.Server{
		Addr:              srv.Minioth.Config.Addr(srv.Minioth.Config.API_PORT),
		Handler:           srv.Engine,
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

/* For this service, authorization is required only for admin role. */
func AuthMiddleware(role string, srv *MService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// if service secret exists and validated, grant access
		if s_secret_claim := c.GetHeader("X-Service-Secret"); s_secret_claim != "" {
			if s_secret_claim == string(srv.Minioth.Config.ServiceSecret) {
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

		// Parse and validate the token
		token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecretKey, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Set claims in the context for further use
		if claims, ok := token.Claims.(*CustomClaims); ok {
			if !strings.Contains(claims.Groups, role) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "invalid user",
				})
				c.Abort()
				return
			}
			c.Set("username", claims.UserID)
			c.Set("groups", claims.Groups)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		c.Next()
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

// jwt
func GenerateAccessJWT(userID, username, groups, gids string) (string, error) {
	// Set the claims for the token
	claims := CustomClaims{
		UserID:   userID,
		Username: username,
		Groups:   groups,
		GroupIDS: gids,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "minioth",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * time.Duration(JWT_VALIDITY_HOURS))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}

	// Create the token using the HS256 signing method
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token using the secret key
	tokenString, err := token.SignedString(jwtSecretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func DecodeJWT(tokenString string) (bool, *CustomClaims, error) {
	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecretKey, nil
	})

	if err != nil || !token.Valid {
		token, err = jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtRefreshKey, nil
		})
	}

	if err != nil {
		log.Printf("%v token, exiting", token)
		return false, nil, err
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		log.Printf("not okay when retrieving claims")
		return false, nil, errors.New("invalid claims")
	}

	return true, claims, nil
}

func GenerateRefreshJWT(userID string) (string, error) {
	claims := CustomClaims{
		UserID: userID,
		Groups: "not-needed",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 72)), // Token expiration time (24 hours)
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtRefreshKey)
}

func groupsToString(groups []ut.Group) string {
	var res []string

	for _, group := range groups {
		res = append(res, group.ToString())
	}

	return strings.Join(res, ",")
}

func gidsToString(groups []ut.Group) string {
	var res []string
	for _, group := range groups {
		res = append(res, strconv.Itoa(group.Gid))
	}
	return strings.Join(res, ",")
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
type CustomClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Groups   string `json:"groups"`
	GroupIDS string `json:"group_ids"`
	jwt.RegisteredClaims
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
