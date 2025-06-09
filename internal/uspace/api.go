// Package uspace details
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

	// swagger documentation
	_ "kyri56xcaesar/kuspace/api/uspace"
	k "kyri56xcaesar/kuspace/internal/uspace/kubernetes"
	ut "kyri56xcaesar/kuspace/internal/utils"
	"kyri56xcaesar/kuspace/pkg/fslite"
)

const (
	version                  = "/v1"
	maxDefaultVolumeCapacity = 100
)

var verbose = true

// UService struct as in the central data structure for the USerivce microservice
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
	// dbh DBHandler

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

// NewUService function as in a constructor for UService struct
/*
		"constructor"

	  - @by shipment it is meant the function that getsOrCreates
	    the object or refence to a system of choice upon choice (configuration)
*/
func NewUService(conf string) UService {
	// configuration
	cfg := ut.LoadConfig(conf)
	verbose = cfg.Verbose

	setGinMode(cfg.APIGinMode)
	// service
	srv := UService{
		Engine: gin.Default(),
		config: cfg,
		// dbh:     NewDBHandler(cfg.DB_RV, cfg.DB_RV_DRIVER),
	}

	// storage system (constructing)
	storage := StorageShipment(strings.ToLower(cfg.StorageSystem), &srv)
	// storage shipment will panic if its not working
	srv.storage = storage

	// dispatcher system (constructing)
	jdp, err := DispatcherShipment(strings.ToLower(cfg.UspaceDispatcher), &srv)
	if err != nil {
		panic(err)
	}
	jobsSocketAddress = cfg.WssAddress
	if jobsSocketAddress == "" {
		panic(errors.New("jobs socket address is empty"))
	}
	srv.jdp = jdp
	jdp.Start() // start "master" worker (the one that spawns other workers)

	// database (init)
	jdbh := ut.NewDBHandler(cfg.UspaceJobsDB, cfg.UspaceJobsDBPath, cfg.UspaceJobsDBDriver)
	srv.jdbh = jdbh
	srv.jdbh.Init(initSQLJobs, cfg.UspaceJobsDBMaxOpenConns, cfg.UspaceJobsDBMaxIdleConns, cfg.UspaceJobsDBMaxLifetime)

	// fsl for storing and enforcing files securly
	copyCfg := cfg.DeepCopy()
	copyCfg.FslLocality = false
	copyCfg.FslServer = false
	copyCfg.FslDB = "fsl_local.db"

	srv.fsl = fslite.NewFsLite(copyCfg)

	// lets create a default bucket
	defaultVolume := ut.Volume{Name: cfg.MinioDefaultBucket, CreatedAt: ut.CurrentTime()}
	err = storage.CreateVolume(defaultVolume)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Printf("[USPACE_init] default volume already exists... continuing")
		} else {
			log.Fatal("[USPACE_init] failed to create the default volume: ", err)
		}
	}

	if verbose {
		log.Printf("[USPACE_init] default bucket ready: %s", cfg.MinioDefaultBucket)
	}

	// store it in local db as well
	err = srv.fsl.CreateVolume(defaultVolume)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Printf("[USPACE_init] default volume already exists in database... continuing")
		} else {
			log.Fatalf("[USPACE_init] failed to save to local fsl db: %v", err)
		}
	}

	return srv
}

// Serve function  launches the server listener
/* listen on http at handled endpoints */
func (srv *UService) Serve() {
	srv.RegisterRoutes()
	/* context handler */
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	/* server std lib raw definition */
	server := &http.Server{
		Addr:              srv.config.Addr(srv.config.APIPort),
		Handler:           srv.Engine,
		ReadHeaderTimeout: time.Second * 5,
	}

	/* listen in a goroutine */
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

// RegisterRoutes method will simply attach the endpoints to the server
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
	apiV1 := srv.Engine.Group("/api" + version)
	apiV1.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.InstanceName("uspacedocs")))
	if strings.ToLower(srv.config.APIGinMode) != "debug" {
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
		/* equivalent to "ls", will
		return the resources, from the given path*/
		apiV1.GET("/resources", srv.getResourcesHandler)
		apiV1.POST("/resource/upload", srv.handleUpload)

		// these endpoints need privileges
		apiV1.GET("/resource/preview", hasAccessMiddleware("r", srv), srv.handlePreview)
		apiV1.GET("/resource/download", hasAccessMiddleware("r", srv), srv.handleDownload)
		apiV1.DELETE("/resource/rm", hasAccessMiddleware("w", srv), srv.rmResourceHandler)
		apiV1.POST("/resource/cp", hasAccessMiddleware("r", srv), srv.cpResourceHandler)
		apiV1.PATCH("/resource/mv", hasAccessMiddleware("w", srv), srv.mvResourcesHandler)
		apiV1.PATCH("/resource/permissions", isOwner(srv), srv.chmodResourceHandler)
		apiV1.PATCH("/resource/ownership", isOwner(srv), srv.chownResourceHandler)
		apiV1.PATCH("/resource/group", isOwner(srv), srv.chgroupResourceHandler)
	}

	admin := srv.Engine.Group("/api" + version + "/admin")

	if strings.ToLower(srv.config.APIGinMode) != "debug" {
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
				kMetrics, err := k.GetSystemMetrics(srv.config.Namespace)
				if err != nil {
					log.Printf("[API] system metrics errors: %v", err)
				}
				c.JSON(http.StatusOK, kMetrics)
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+srv.config.AuthAddress+":"+srv.config.AuthPort+"/v1/admin/groups", nil)
	if err != nil {
		log.Printf("failed to create a request: %v", err)

		return err
	}
	req.Header.Add("X-Service-Secret", string(srv.config.ServiceSecretKey))
	var reqR struct {
		Content []ut.Group `json:"content"`
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to do request: %v", err)

		return err
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	if err := json.NewDecoder(resp.Body).Decode(&reqR); err != nil {
		log.Printf("failed to decode response body: %v", err)

		return err
	}
	if len(reqR.Content) == 0 {
		log.Printf("request returned empty slice of users, false condition")

		return errors.New("failed to retrieve actual users")
	}

	capacity := min(srv.config.LocalVolumesDefaultCapacity, maxDefaultVolumeCapacity)

	// we retrieved the users, lets add the users volume claims and the corresponding primary group claims
	for _, group := range reqR.Content {
		if group.Groupname == "admin" || group.Groupname == "user" || group.Groupname == "mod" {
			continue
		}

		cancelFn, err := srv.storage.Insert([]any{ut.GroupVolume{
			VID:   1,
			GID:   group.GID,
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
					VID:   1,
					UID:   user.UID,
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
