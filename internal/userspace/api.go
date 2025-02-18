package userspace

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	ut "kyri56xcaesar/myThesis/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

/* Structure containing all needed aspects of this service */
type UService struct {
	/* operator logic of the service
	*   -> perhaps this should be attachable <-
	* */
	/* server engine */
	Engine *gin.Engine

	/* database calls functions  */
	dbh DBHandler

	/* configuration file (.env) */
	config ut.EnvConfig
}

/* "constructor" */
func NewUService(conf string) UService {
	// configuration
	cfg := ut.LoadConfig(conf)

	// service
	srv := UService{
		Engine: gin.Default(),
		config: cfg,
		dbh: DBHandler{
			DBName: cfg.DB,
		},
	}

	// datbase
	srv.dbh.Init(cfg.DBPath, cfg.Volumes, cfg.VCapacity)

	// also ensure local pv path
	_, err := os.Stat(cfg.Volumes)
	if err != nil {
		err = os.Mkdir(cfg.Volumes, 0o777)
		if err != nil {
			panic("crucial")
		}
	}

	return srv
}

/* listen on http at handled endpoints */
func (srv *UService) Serve() {
	root := srv.Engine.Group("/")
	{
		root.GET("/healthz", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "alive",
			})
		})
	}
	/* These endpoints should parse an authentication token and handle verification of authorization according
	*    to the permissions of the user. For now, we will just implement the endpoints without any
	* */
	apiV1 := srv.Engine.Group("/api/v1")
	apiV1.Use(serviceAuth(srv))
	{
		/* equivalent to "ls", will return the resources, from the given path*/
		apiV1.GET("/resources", srv.ResourcesHandler)

		apiV1.POST("/resource/upload", srv.HandleUpload)
		apiV1.GET("/resource/download", hasAccessMiddleware("r", srv), srv.HandleDownload)
		apiV1.GET("/resource/preview", hasAccessMiddleware("r", srv), srv.HandlePreview)

		apiV1.DELETE("/resource/rm", hasAccessMiddleware("w", srv), srv.RemoveResourceHandler)
		apiV1.PATCH("/resource/mv", hasAccessMiddleware("w", srv), srv.MoveResourcesHandler)
		apiV1.POST("/resource/cp", hasAccessMiddleware("r", srv), srv.ResourceCpHandler)

		apiV1.PATCH("/resource/permissions", isOwner(srv), srv.ChmodResourceHandler)
		apiV1.PATCH("/resource/ownership", isOwner(srv), srv.ChownResourceHandler)
		apiV1.PATCH("/resource/group", isOwner(srv), srv.ChgroupResourceHandler)

	}

	admin := apiV1.Group("/admin")
	admin.Use(serviceAuth(srv))
	{
		admin.POST("/resources", srv.PostResourcesHandler)

		admin.GET("/volumes", srv.HandleVolumes)
		admin.POST("/volumes", srv.HandleVolumes)
		admin.PUT("/volumes", srv.HandleVolumes)
		admin.DELETE("/volumes", srv.HandleVolumes)
		admin.PATCH("/volumes", srv.HandleVolumes)

		admin.GET("/user/volume", srv.HandleUserVolumes)
		admin.POST("/user/volume", srv.HandleUserVolumes)
		admin.PATCH("/user/volume", srv.HandleUserVolumes)
		admin.DELETE("/user/volume", srv.HandleUserVolumes)

		admin.GET("/group/volume", srv.HandleGroupVolumes)
		admin.POST("/group/volume", srv.HandleGroupVolumes)
		admin.PATCH("/group/volume", srv.HandleGroupVolumes)
		admin.DELETE("/group/volume", srv.HandleGroupVolumes)

	}
	/* context handler */
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	/* server std lib raw definition */
	server := &http.Server{
		Addr:              srv.config.Addr(srv.config.API_PORT),
		Handler:           srv.Engine,
		ReadHeaderTimeout: time.Second * 5,
	}

	/* listen in a goroutine */
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	/* handle signals on process ..
	*
	* */
	<-ctx.Done()

	stop()
	log.Println("shutting down gracefully, press Ctrl+C again to force")

	/* don't forgt to close the db conn */
	srv.dbh.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}

/*
	take this function to forge a jwt token for minioth

This service wants to have admin access to minioth
*/
func (u *UService) generateAccessJWT(userID, username, groups, gids string) (string, error) {
	type CustomClaims struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
		Groups   string `json:"groups"`
		GroupIDS string `json:"group_ids"`
		jwt.RegisteredClaims
	}
	// Set the claims for the token
	claims := CustomClaims{
		UserID:   userID,
		Username: username,
		Groups:   groups,
		GroupIDS: gids,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "minioth",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * 10)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}

	// Create the token using the HS256 signing method
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token using the secret key
	tokenString, err := token.SignedString(u.config.JWTSecretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}
