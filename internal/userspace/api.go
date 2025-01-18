package userspace

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	ut "kyri56xcaesar/myThesis/internal/utils"

	"github.com/gin-gonic/gin"
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
	srv.dbh.Init()

	// also ensure local pv path
	_, err := os.Stat(cfg.Volumes)
	if err != nil {
		err = os.Mkdir(cfg.Volumes, 0700)
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
	{
		/* equivalent to "ls", will return the resources, from the given path*/
		apiV1.GET("/resources", srv.ResourcesHandler)

		apiV1.POST("/resource/upload", srv.HandleUpload)
		apiV1.GET("/resource/download", srv.HandleDownload)
	}

	admin := apiV1.Group("/admin")
	{

		admin.POST("/resources", srv.PostResourcesHandler)
		admin.PUT("/resources", srv.MoveResourcesHandler)

		admin.PATCH("/resource/permissions", srv.ChmodResourceHandler)
		admin.PATCH("/resource/ownership", srv.ChownResourceHandler)
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
