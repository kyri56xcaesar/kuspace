package userspace

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	ut "kyri56xcaesar/myThesis/internal/utils"

	"github.com/gin-gonic/gin"
)

/* Structure containing all needed aspects of this service */
type UService struct {
	/* configuration file (.env) */
	config ut.EnvConfig

	/* server engine */
	Engine *gin.Engine

	/* database calls handlers for resources/volumes database */
	dbh DBHandler

	/* database calls handlers for jobs database*/
	dbhJobs DBHandler

	/* a job dispatcher: a preparation to runming jobs*/
	jdp JobDispatcher
}

/* "constructor" */
func NewUService(conf string) UService {
	// configuration
	cfg := ut.LoadConfig(conf)

	// service
	srv := UService{
		Engine:  gin.Default(),
		config:  cfg,
		dbh:     NewDBHandler(cfg.DB_RV, cfg.DB_RV_DRIVER),
		dbhJobs: NewDBHandler(cfg.DB_JOBS, cfg.DB_JOBS_DRIVER),
	}

	// dispatcher
	jdp, err := DispatcherFactory(strings.ToLower(cfg.J_DISPATCHER))
	if err != nil {
		panic(err)
	}
	srv.jdp = jdp
	jdp.Start()

	// datbase
	srv.dbh.Init(initSql, cfg.DB_RV_Path, cfg.DB_RV_MAX_OPEN_CONNS, cfg.DB_RV_MAX_IDLE_CONNS, cfg.DB_RV_MAX_LIFETIME)
	srv.dbhJobs.Init(initSqlJobs, cfg.DB_JOBS_Path, cfg.DB_JOBS_MAX_OPEN_CONNS, cfg.DB_JOBS_MAX_IDLE_CONNS, cfg.DB_JOBS_MAX_LIFETIME)

	// some specific init
	srv.dbh.InitResourceVolumeSpecific(cfg.DB_RV_Path, cfg.Volumes, cfg.VCapacity)

	// also ensure local pv path
	_, err = os.Stat(cfg.Volumes)
	if err != nil {
		err = os.Mkdir(cfg.Volumes, 0o777)
		if err != nil {
			panic("crucial")
		}
	}

	// we should init sync (if there are) existing users (from a different service)
	go func() {
		err := syncUsers(&srv)
		if err != nil && strings.Contains(err.Error(), "already exists") {
			log.Printf("users are in sync (already).")
		} else if err != nil {
			log.Printf("syncUsers failed: %v", err)
		} else {
			log.Println("users synced in userspace (uservolumes,groupvolumes claims).")
		}
	}()

	log.Printf("server at: %+v", srv)
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

		// job related
		apiV1.POST("/job", srv.HandleJob)
		apiV1.GET("/job", srv.HandleJob)

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

		admin.GET("/job", srv.HandleJobAdmin)
		admin.POST("/job", srv.HandleJobAdmin)
		admin.DELETE("/job", srv.HandleJobAdmin)
		admin.PUT("/job", srv.HandleJobAdmin)

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
	srv.dbhJobs.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}

func syncUsers(srv *UService) error {
	req, err := http.NewRequest(http.MethodGet, "http://"+srv.config.AUTH_ADDRESS+":"+srv.config.AUTH_PORT+"/admin/groups", nil)
	if err != nil {
		log.Printf("failed to create a request: %v", err)
		return err
	}
	req.Header.Add("X-Service-Secret", string(srv.config.ServiceSecret))
	var reqR struct {
		Content []Group `json:"content"`
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to do request: %v", err)
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&reqR); err != nil {
		log.Printf("failed to decode response body: %v", err)
		return err
	}
	if len(reqR.Content) == 0 {
		log.Printf("request returned empty slice of users, false condition")
		return fmt.Errorf("failed to retrieve actual users")
	}

	capacity, err := strconv.ParseFloat(srv.config.VCapacity, 64)
	if err != nil {
		log.Printf("failed to parse env var VCapacity: %v", err)
		return err
	}
	// we retrieved the users, lets add the users volume claims and the corresponding primary group claims
	for _, group := range reqR.Content {
		if group.Groupname == "admin" || group.Groupname == "user" || group.Groupname == "mod" {
			continue
		}

		err := srv.dbh.InsertGroupVolume(GroupVolume{
			Vid:   1,
			Gid:   group.Gid,
			Quota: capacity,
		})
		if err != nil {
			log.Printf("failed to insert gv: %v", err)
			return err
		}
		for _, user := range group.Users {
			if user.Username == group.Groupname {
				err := srv.dbh.InsertUserVolume(UserVolume{
					Vid:   1,
					Uid:   user.Uid,
					Quota: capacity,
				})
				if err != nil {
					log.Printf("failed to insert uv: %v", err)
					return err
				}
			}
		}
	}
	return nil
}
