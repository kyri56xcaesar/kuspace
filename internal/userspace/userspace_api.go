package userspace

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

type UService struct {
	Engine *gin.Engine
	Config *EnvConfig
	dbh    *DBHandler
}

func NewUService(conf string) *UService {
	cfg := LoadConfig(conf)
	srv := &UService{
		Engine: gin.Default(),
		Config: cfg,
		dbh: &DBHandler{
			DBName: cfg.DB,
		},
	}

	log.Print(srv.Config.ToString())

	srv.dbh.Init()

	return srv
}

func (srv *UService) Serve() {
	/* These endpoints should parse an authentication token and handle verification of authorization according
	*    to the permissions of the user. For now, we will just implement the endpoints without any
	* */
	apiV1 := srv.Engine.Group("/api/v1")
	{
		apiV1.GET("/files", srv.GetFiles)

		/* for these, i should check for permissions*/
		// post needs write permission on parent
		apiV1.POST("/files", srv.PostFile)
		// patch needs write permission on parent and on the resource
		apiV1.PATCH("/files/:id", srv.PatchFile)
		// same as patch
		apiV1.DELETE("/files/:id", srv.DeleteFiles)

	}

	admin := srv.Engine.Group("/admin")
	{
		admin.GET("/files", srv.GetFiles)
	}

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

	/* don't forgt to close the db conn */
	srv.dbh.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}
