package userspace

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	ut "kyri56xcaesar/myThesis/internal/utils"

	"github.com/gin-gonic/gin"
)

const (
	VERSION = "/v1"
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

	// service
	srv := UService{
		Engine: gin.Default(),
		config: cfg,
		//dbh:     NewDBHandler(cfg.DB_RV, cfg.DB_RV_DRIVER),
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
	jdp.Start()

	// database (init)
	jdbh := ut.NewDBHandler(cfg.DB_JOBS, cfg.DB_JOBS_PATH, cfg.DB_JOBS_DRIVER)
	srv.jdbh = jdbh
	srv.jdbh.Init(initSqlJobs, cfg.DB_JOBS_MAX_OPEN_CONNS, cfg.DB_JOBS_MAX_IDLE_CONNS, cfg.DB_JOBS_MAX_LIFETIME)

	//srv.dbh.Init(initSql, cfg.DB_RV_Path, cfg.DB_RV_MAX_OPEN_CONNS, cfg.DB_RV_MAX_IDLE_CONNS, cfg.DB_RV_MAX_LIFETIME)

	// some specific init
	//srv.dbh.InitResourceVolumeSpecific(cfg.DB_RV_Path, cfg.Volumes, cfg.VCapacity)

	// also ensure local pv path
	//_, err = os.Stat(cfg.Volumes)
	//if err != nil {
	//	err = os.Mkdir(cfg.Volumes, 0o777)
	//	if err != nil {
	//		panic("crucial")
	//	}
	//}

	// we should init sync (if there are) existing users (from a different service)
	// go func() {
	// 	err := syncUsers(&srv)
	// 	if err != nil && strings.Contains(err.Error(), "already exists") {
	// 		log.Printf("users are in sync (already).")
	// 	} else if err != nil {
	// 		log.Printf("syncUsers failed: %v", err)
	// 	} else {
	// 		log.Println("users synced in userspace (uservolumes,groupvolumes claims).")
	// 	}
	// }()

	// log.Printf("server at: %+v", srv)
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
	apiV1 := srv.Engine.Group("/api" + VERSION)
	// apiV1.Use(serviceAuth(srv)) //, bindHeadersMiddleware())
	apiV1.Use(bindHeadersMiddleware())
	{
		/* equivalent to "ls", will return the resources, from the given path*/
		apiV1.GET("/resources", srv.getResourcesHandler)

		apiV1.POST("/resource/upload", srv.handleUpload)
		apiV1.GET("/resource/download", srv.handleDownload)
		apiV1.GET("/resource/preview", srv.handlePreview)
		// apiV1.GET("/resource/download", hasAccessMiddleware("r", srv), srv.handleDownload)

		// apiV1.DELETE("/resource/rm", hasAccessMiddleware("w", srv), srv.rmResourceHandler)
		// apiV1.PATCH("/resource/mv", hasAccessMiddleware("w", srv), srv.mvResourcesHandler)
		// apiV1.POST("/resource/cp", hasAccessMiddleware("r", srv), srv.cpResourceHandler)
		apiV1.DELETE("/resource/rm", srv.rmResourceHandler)
		apiV1.PATCH("/resource/mv", srv.mvResourcesHandler)
		apiV1.POST("/resource/cp", srv.cpResourceHandler)

		// apiV1.PATCH("/resource/permissions", isOwner(srv), srv.chmodResourceHandler)
		// apiV1.PATCH("/resource/ownership", isOwner(srv), srv.chownResourceHandler)
		// apiV1.PATCH("/resource/group", isOwner(srv), srv.chgroupResourceHandler)

		// job related
		apiV1.Match(
			[]string{"GET", "POST"},
			"/job",
			srv.HandleJob,
		)
	}

	admin := srv.Engine.Group("/api" + VERSION + "/admin")
	// admin.Use(serviceAuth(srv)) //, bindHeadersMiddleware())
	{
		admin.Match(
			[]string{"GET", "POST", "PUT", "DELETE", "PATCH"},
			"/volumes",
			srv.HandleVolumes,
		)
		admin.Match(
			[]string{"DELETE", "PUT"},
			"/job",
			srv.HandleJobAdmin,
		)
		// admin.Match(
		// 	[]string{"GET", "POST", "PATCH", "DELETE"},
		// 	"/user/volume",
		// 	srv.HandleUserVolumes,
		// )
		// admin.Match(
		// 	[]string{"GET", "POST", "PATCH", "DELETE"},
		// 	"/group/volume",
		// 	srv.HandleGroupVolumes,
		// )
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
	srv.jdbh.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}

func syncUsers(srv *UService) error {
	req, err := http.NewRequest(http.MethodGet, "http://"+srv.config.AUTH_ADDRESS+":"+srv.config.AUTH_PORT+"/v1/admin/groups", nil)
	if err != nil {
		log.Printf("failed to create a request: %v", err)
		return err
	}
	req.Header.Add("X-Service-Secret", string(srv.config.ServiceSecret))
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

	capacity, err := strconv.ParseFloat(srv.config.LOCAL_VOLUMES_DEFAULT_CAPACITY, 64)
	if err != nil {
		log.Printf("failed to parse env var VCapacity: %v", err)
		return err
	}
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
