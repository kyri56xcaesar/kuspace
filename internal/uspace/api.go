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
	"fmt"
	"kyri56xcaesar/kuspace/internal/authn"
	"log"
	"net/http"
	"os/signal"
	"runtime"
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

	// tokens verifies minioth access tokens (identity.go, tokens.go)
	tokens *authn.Verifier

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
	cfg := ut.LoadConfig(conf, ut.Service, ut.Peers, ut.Auth, ut.Tokens, ut.Storage, ut.Fslite, ut.Minio, ut.Jobs)
	verbose = cfg.Verbose

	setGinMode(cfg.APIGinMode)
	// service
	srv := UService{
		Engine: gin.Default(),
		config: cfg,
		tokens: authn.NewVerifier(cfg),
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
	if err := srv.ensureJobColumns(context.Background()); err != nil {
		log.Printf("[USPACE_init] failed to add new jobs columns: %v", err)
	}
	if err := srv.seedDefaultApps(context.Background()); err != nil {
		log.Printf("[USPACE_init] failed to install default apps: %v", err)
	}

	// fsl for storing and enforcing files securly
	copyCfg := cfg.DeepCopy()
	copyCfg.FslLocality = false
	copyCfg.FslServer = false
	copyCfg.FslDB = "fsl_local.db"

	srv.fsl = fslite.NewFsLite(copyCfg)

	// lets create a default bucket
	defaultVolume := ut.Volume{Name: cfg.MinioDefaultBucket, CreatedAt: ut.CurrentTime()}
	err = storage.CreateVolume(context.Background(), defaultVolume)
	if err != nil {
		if errors.Is(err, ut.ErrExists) {
			log.Printf("[USPACE_init] default volume already exists... continuing")
		} else {
			log.Fatal("[USPACE_init] failed to create the default volume: ", err)
		}
	}

	if verbose {
		log.Printf("[USPACE_init] default bucket ready: %s", cfg.MinioDefaultBucket)
	}

	// store it in local db as well
	err = srv.fsl.CreateVolume(context.Background(), defaultVolume)
	if err != nil {
		if errors.Is(err, ut.ErrExists) {
			log.Printf("[USPACE_init] default volume already exists in database... continuing")
		} else {
			log.Fatalf("[USPACE_init] failed to save to local fsl db: %v", err)
		}
	}

	syncUsersInBackground(&srv)

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
	server := ut.ServerTimeouts(&http.Server{
		Addr:    srv.config.Addr(srv.config.APIPort),
		Handler: srv.Engine,
	})

	/* listen in a goroutine */
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[USPACE_SERVER] listen: %s\n", err)
		}
	}()
	<-ctx.Done()

	stop()
	log.Println("[USPACE_SERVER] shutting down gracefully, press Ctrl+C again to force")

	// in order: finish the HTTP requests in flight, let running jobs finish
	// (they record their output and status), then close the databases.
	// (The databases used to be closed first, failing the requests still
	// being served.)
	shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutCtx); err != nil {
		log.Printf("[USPACE_SERVER] requests cut short: %v", err)
	}
	drainCtx, cancelDrain := context.WithTimeout(context.WithoutCancel(ctx), jobDrainTimeout)
	defer cancelDrain()
	if err := srv.jdp.Drain(drainCtx); err != nil {
		log.Printf("[USPACE_SERVER] stopped waiting for jobs: %v", err)
	}
	srv.jdbh.Close()
	srv.fsl.Close()

	log.Println("[USPACE_SERVER] Server exiting")
}

// readinessChecks: what uspace needs to serve requests.
func (srv *UService) readinessChecks() map[string]ut.Check {
	return map[string]ut.Check{
		"metadata": srv.fsl.Ping,
		"jobs": func(ctx context.Context) error {
			db, err := srv.jdbh.GetConn()
			if err != nil {
				return err
			}

			return db.PingContext(ctx)
		},
		"storage": func(ctx context.Context) error {
			_, err := srv.storage.SelectVolumes(ctx, nil)

			return err
		},
	}
}

// jobDrainTimeout bounds how long shutdown waits for running jobs
// (kubernetes gives pods 30s by default after SIGTERM; stay inside it).
const jobDrainTimeout = 15 * time.Second

// RegisterRoutes method will simply attach the endpoints to the server
func (srv *UService) RegisterRoutes() {
	root := srv.Engine.Group("/")
	{
		root.GET("/healthz", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "alive",
			})
		})
		// ready = able to serve: its databases and the object store answer
		root.GET("/readyz", func(c *gin.Context) {
			status, checks := ut.Ready(c.Request.Context(), srv.readinessChecks())
			c.JSON(status, checks)
		})
	}
	/* These endpoints should parse an authentication token and handle verification of authorization according
	*    to the permissions of the user. For now, we will just implement the endpoints without any
	* */
	apiV1 := srv.Engine.Group("/api" + version)
	apiV1.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.InstanceName("uspacedocs")))
	// every call is authenticated (see identity.go): a user's minioth access
	// token, or the service secret. (Auth used to be skipped entirely in gin
	// "debug" mode.)
	apiV1.Use(authenticate(srv))
	{
		// jobs can be run from anyone
		// job related
		apiV1.Match(
			[]string{"GET", "POST"},
			"/job",
			srv.handleJob,
		)
		apiV1.GET("/job/log", srv.handleJobLog)
		apiV1.POST("/job/cancel", srv.handleJobCancel)
		apiV1.Match(
			[]string{"GET", "POST"},
			"/app",
			srv.handleApps,
		)
		apiV1.Use(bindHeadersMiddleware())
		/* equivalent to "ls", will
		return the resources, from the given path*/
		apiV1.GET("/resources", srv.getResourcesHandler)
		apiV1.GET("/volumes/shared", srv.handleSharedVolumes)
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

	admin.Use(authenticateAdmin(srv), bindHeadersMiddleware())
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
			[]string{"GET", "POST", "PATCH", "DELETE"},
			"/group/volume",
			srv.handleGroupVolumes,
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
				// uspace's own process, available with or without a cluster
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				kMetrics["process"] = gin.H{"goroutines": runtime.NumGoroutine(), "heap_bytes": ms.HeapAlloc}
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

// syncUsers gives every existing minioth user a claim on the default volume.
// Registration creates these claims, but users registered before that worked
// (or while uspace was down) have none. Safe to run repeatedly: existing
// claims are left alone.
func syncUsers(srv *UService) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+srv.config.AuthAddress+":"+srv.config.AuthPort+"/v1/admin/groups", nil)
	if err != nil {
		return fmt.Errorf("failed to create a request: %w", err)
	}
	req.Header.Set("X-Service-Secret", string(srv.config.ServiceSecretKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach minioth: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("minioth answered %s", resp.Status)
	}
	var groups struct {
		Content []ut.Group `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		return fmt.Errorf("failed to decode groups: %w", err)
	}

	v, err := srv.fsl.SelectVolumes(ctx, map[string]any{"name": srv.config.MinioDefaultBucket})
	if err != nil {
		return fmt.Errorf("default volume: %w", err)
	}
	volume, ok := v.(ut.Volume)
	if !ok {
		return errors.New("default volume: unexpected type")
	}
	quota := min(srv.config.LocalVolumesDefaultCapacity, maxDefaultVolumeCapacity)

	claimed := 0
	// a user's primary group is the one named after them
	for _, group := range groups.Content {
		for _, user := range group.Users {
			if user.Username != group.Groupname {
				continue
			}
			err := srv.fsl.Insert(ctx, ut.UserVolume{VID: volume.VID, UID: user.UID, Quota: quota})
			switch {
			case err == nil:
				claimed++
			case errors.Is(err, ut.ErrExists):
			default:
				log.Printf("[USPACE_sync] failed to claim volume for uid %d: %v", user.UID, err)
			}
		}
	}
	log.Printf("[USPACE_sync] volume claims created for %d existing user(s)", claimed)

	return nil
}

// syncUsersInBackground retries syncUsers until minioth is reachable, without
// holding up start-up.
func syncUsersInBackground(srv *UService) {
	go func() {
		for attempt, wait := 1, 2*time.Second; attempt <= 6; attempt, wait = attempt+1, wait*2 {
			err := syncUsers(srv)
			if err == nil {
				return
			}
			log.Printf("[USPACE_sync] attempt %d failed: %v", attempt, err)
			time.Sleep(wait)
		}
	}()
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
