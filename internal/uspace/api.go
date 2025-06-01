// @title           Uspace API
// @version         1.0
// @description     API for submitting/monitoring jobs to/from an execution machine
// @host            localhost:8079
// @BasePath        /api/v1
// @schemes         http
package uspace

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "kyri56xcaesar/kuspace/api/uspace"
	k "kyri56xcaesar/kuspace/internal/uspace/kubernetes"
	ut "kyri56xcaesar/kuspace/internal/utils"
	"kyri56xcaesar/kuspace/pkg/fslite"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const (
	VERSION                     = "/v1"
	MAX_DEFAULT_VOLUME_CAPACITY = 100
)

var (
	verbose bool = true
)

/*
	Structure containing all needed aspects of this service

- configuration object (ofc)
- http engine (gin) reference
- a storage system for files reference
- a local database + handler system for the Jobs
- a Job "dispatcher" system reference
*/
type UService struct {
	/* configuration file (.env) */
	config ut.EnvConfig

	/* server engine */
	Engine *gin.Engine

	/* database calls handlers for resources/volumes database */
	//dbh DBHandler

	/* a storage system that this service is gonna use
	it can be either a basic volume occupation
	or a minio storage system
	or anything else implementin this interface
	*/
	storage StorageSystem

	/* database call handlers for the Jobs db*/
	jdbh ut.DBHandler

	// a database related to files
	// to enforce security on ownerships
	fsl fslite.FsLite

	/* a job dispatcher: a scheduling/setup/preparation system for runming jobs*/
	/* it is directly associated to other objects, JobManager, JobExecutor that
	eventually carry out the execution.
	*/
	jdp JobDispatcher
}

/*
		"constructor"

	  - @by shipment it is meant the function that getsOrCreates
	    the object or refence to a system of choice upon choice (configuration)
*/
func NewUService(conf string) UService {
	// configuration
	cfg := ut.LoadConfig(conf)

	setGinMode(cfg.API_GIN_MODE)
	// service
	srv := UService{
		Engine: gin.Default(),
		config: cfg,
		// dbh:     NewDBHandler(cfg.DB_RV, cfg.DB_RV_DRIVER),
	}

	// storage system (constructing)
	storage := StorageShipment(strings.ToLower(cfg.STORAGE_SYSTEM), &srv)
	// storage shipment will panic if its not working
	srv.storage = storage

	// dispatcher system (constructing)
	jdp, err := DispatcherShipment(strings.ToLower(cfg.J_DISPATCHER), &srv)
	if err != nil {
		panic(err)
	}
	jobs_socket_address = cfg.J_WS_ADDRESS
	if jobs_socket_address == "" {
		panic(fmt.Errorf("jobs socket address is empty"))
	}
	srv.jdp = jdp
	jdp.Start() // start "master" worker (the one that spawns other workers)

	// database (init)
	jdbh := ut.NewDBHandler(cfg.DB_JOBS, cfg.DB_JOBS_PATH, cfg.DB_JOBS_DRIVER)
	srv.jdbh = jdbh
	srv.jdbh.Init(initSqlJobs, cfg.DB_JOBS_MAX_OPEN_CONNS, cfg.DB_JOBS_MAX_IDLE_CONNS, cfg.DB_JOBS_MAX_LIFETIME)

	// fsl for storing and enforcing files securly
	copy_cfg := cfg.DeepCopy()
	copy_cfg.FSL_LOCALITY = false
	copy_cfg.FSL_SERVER = false
	copy_cfg.DB_FSL = "fsl_local.db"

	srv.fsl = fslite.NewFsLite(copy_cfg)

	// lets create a default bucket
	default_volume := ut.Volume{Name: cfg.MINIO_DEFAULT_BUCKET, CreatedAt: ut.CurrentTime()}
	err = storage.CreateVolume(default_volume)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Printf("[USPACE_init] default volume already exists... continuing")
		} else {
			log.Fatal("[USPACE_init] failed to create the default volume: ", err)
		}
	}
	log.Printf("[USPACE_init] default bucket ready: %s", cfg.MINIO_DEFAULT_BUCKET)

	// store it in local db as well
	err = srv.fsl.CreateVolume(default_volume)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Printf("[USPACE_init] default volume already exists in database... continueing")
		} else {
			log.Fatalf("[USPACE_init] failed to save to local fsl db: %v", err)
		}
	}

	return srv
}

/* listen on http at handled endpoints */
func (srv *UService) Serve() {
	srv.RegisterRoutes()
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
			log.Fatalf("[USPACE_SERVER] listen: %s\n", err)
		}
	}()
	<-ctx.Done()

	stop()
	log.Println("[USPACE_SERVER] shutting down gracefully, press Ctrl+C again to force")

	/* don't forgt to close the db conn */
	srv.jdbh.Close()
	srv.fsl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("[USPACE_SERVER] Server forced to shutdown: ", err)
	}

	log.Println("[USPACE_SERVER] Server exiting")
}

func (srv *UService) RegisterRoutes() {
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
	apiV1 := srv.Engine.Group("/api" + VERSION)
	apiV1.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.InstanceName("uspacedocs")))
	if strings.ToLower(srv.config.API_GIN_MODE) != "debug" {
		apiV1.Use(serviceAuth(srv))
	}
	{
		// jobs can be run from anyone
		// job related
		apiV1.Match(
			[]string{"GET", "POST"},
			"/job",
			srv.handleJob,
		)
		apiV1.Match(
			[]string{"GET", "POST"},
			"/app",
			srv.handleApps,
		)
		apiV1.Use(bindHeadersMiddleware())
		/* equivalent to "ls", will return the resources, from the given path*/
		apiV1.GET("/resources", srv.getResourcesHandler)
		apiV1.POST("/resource/upload", srv.handleUpload)

		// these endpoints need privelleges
		apiV1.GET("/resource/preview", hasAccessMiddleware("r", srv), srv.handlePreview)
		apiV1.GET("/resource/download", hasAccessMiddleware("r", srv), srv.handleDownload)
		apiV1.DELETE("/resource/rm", hasAccessMiddleware("w", srv), srv.rmResourceHandler)
		apiV1.POST("/resource/cp", hasAccessMiddleware("r", srv), srv.cpResourceHandler)
		apiV1.PATCH("/resource/mv", hasAccessMiddleware("w", srv), srv.mvResourcesHandler)
		apiV1.PATCH("/resource/permissions", isOwner(srv), srv.chmodResourceHandler)
		apiV1.PATCH("/resource/ownership", isOwner(srv), srv.chownResourceHandler)
		apiV1.PATCH("/resource/group", isOwner(srv), srv.chgroupResourceHandler)
	}

	admin := srv.Engine.Group("/api" + VERSION + "/admin")

	if strings.ToLower(srv.config.API_GIN_MODE) != "debug" {
		admin.Use(serviceAuth(srv), bindHeadersMiddleware())
	}
	{
		admin.Match(
			[]string{"GET", "POST", "PUT", "DELETE", "PATCH"},
			"/volumes",
			srv.handleVolumes,
		)
		admin.Match(
			[]string{"DELETE", "PUT"},
			"/job",
			srv.handleJobAdmin,
		)
		admin.Match(
			[]string{"GET", "POST", "PATCH", "DELETE"},
			"/user/volume",
			srv.handleUserVolumes,
		)

		admin.Match(
			[]string{"GET", "POST", "PUT", "DELETE"},
			"/app",
			srv.handleAppsAdmin,
		)
		// system, metrics, conf
		{
			admin.Match(
				[]string{"GET"},
				"/system-conf",
				srv.handleSysConf,
			)

			admin.GET("/system-metrics", func(c *gin.Context) {
				k_metrics, err := k.GetSystemMetrics(srv.config.NAMESPACE)
				if err != nil {
					log.Printf("[API] system metrics errors: %v", err)
				}
				c.JSON(http.StatusOK, k_metrics)
			})

		}
	}
}

func (srv *UService) handleSysConf(c *gin.Context) {
	uspacecfg, err := ut.ReadConfig("configs/"+srv.config.ConfigPath, false)
	if err != nil {
		log.Printf("[API_sysConf] failed to read config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, uspacecfg)
}

// this should propably sync users/data from minio or other storage providers.
func syncUsers(srv *UService) error {
	req, err := http.NewRequest(http.MethodGet, "http://"+srv.config.AUTH_ADDRESS+":"+srv.config.AUTH_PORT+"/v1/admin/groups", nil)
	if err != nil {
		log.Printf("failed to create a request: %v", err)
		return err
	}
	req.Header.Add("X-Service-Secret", string(srv.config.SERVICE_SECRET_KEY))
	var reqR struct {
		Content []ut.Group `json:"content"`
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

	capacity := min(srv.config.LOCAL_VOLUMES_DEFAULT_CAPACITY, MAX_DEFAULT_VOLUME_CAPACITY)

	// we retrieved the users, lets add the users volume claims and the corresponding primary group claims
	for _, group := range reqR.Content {
		if group.Groupname == "admin" || group.Groupname == "user" || group.Groupname == "mod" {
			continue
		}

		cancelFn, err := srv.storage.Insert([]any{ut.GroupVolume{
			Vid:   1,
			Gid:   group.Gid,
			Quota: capacity,
		}})
		defer cancelFn()
		if err != nil {
			log.Printf("failed to insert gv: %v", err)
			return err
		}
		for _, user := range group.Users {
			if user.Username == group.Groupname {
				cancelFn, err := srv.storage.Insert([]any{ut.UserVolume{
					Vid:   1,
					Uid:   user.Uid,
					Quota: capacity,
				}})
				defer cancelFn()
				if err != nil {
					log.Printf("failed to insert uv: %v", err)
					return err
				}

			}
		}
	}
	return nil
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
