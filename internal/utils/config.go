package utils

/*
* part of the utils module
*
*	a Central struct and its methods
* 	reusable code for extracting/loading/viewing configuration variables.
*
*	There is a central struct EnvConfig which holds all possible variable
*	used by the services. There are some overlaps in need therefore we have
* 	only 1 wholesome type.
*
*	Perhaps in the future we can divide into more atomic configuration structs...
*	works for now.
*
*
* */

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// EnvConfig struct holds all info needed for the microservices configurations
type EnvConfig struct {
	ConfigPath string // path of the .conf file
	Profile    string // baremeta or container
	Verbose    bool
	AsOperator bool   // not used rn
	Namespace  string // kubernetes namespace deployment

	// ###################################
	// API CONFS
	// api as a general api (used for uspace)
	IP                 string
	Issuer             string
	APIUseTLS          bool
	APICertFile        string // path to a cert file
	APIKeyFile         string // path to a key file
	APIGinMode         string
	APILogsPath        string // path to logs dir
	APILogsMaxFetch    int    // max logs size (in MB)
	APIPort            string // main service port
	APIAddress         string // main service address name (default IP)
	FrontPort          string
	FrontAddress       string
	AuthPort           string
	AuthAddress        string
	WssAddressInternal string
	// front as in the frontend app  // frontend app itself should use api vars
	// auth as in an authentication app // if an auth app uses this configuration, it should reference api port as itself

	// service (main) authentication info
	Jwks             string // used by (minioth)
	JwtValidityHours float64
	JwtSecretKey     []byte
	ServiceSecretKey []byte
	AllowedOrigins   []string
	AllowedHeaders   []string
	AllowedMethods   []string
	HashCost         string // used by (minioth) //bcrypt

	// ###################################
	// storage/jobs
	// ###################################
	StorageSystem string // a storage system the service might use

	// 1. can use fslite
	// conf
	FslDB             string // name of the database
	FslDBPath         string // path of the database
	FslDBDriver       string // type of database driver, either duckdb or sqlite3
	FslDBMaxOpenConns string // maximum allowed simutanious open connections
	FslDBMaxIdleConns string // maximum allowed idle connections
	FslDBMaxLifetime  string // lifetime of an idle connection
	FslAccessKey      string // "root" or admin username for authentication
	FslSecretKey      string // "root" or admin password for authentication
	FslServer         bool
	FslLocality       bool
	FslUnlocked       bool // Unlock means, don't limit or check for usage/capacity per volume

	// fslite uses a local data directory for storage
	LocalVolumesDefaultPath     string  // path to the data directory
	LocalVolumesDefaultCapacity float64 // default capacity of the storage

	// 2. can use minio
	// conf
	MinioNodeportEndpoint   string // minio API endpoint if running inside kube and exposed via nodeport
	MinioEndpoint           string // minio API endpoint
	MinioAccessKey          string // "root" or admin username for authentication
	MinioSecretKey          string // "root" or admin password for authentication
	MinioUseSSL             string // boolean (if https or not)
	MinioDefaultBucket      string // default bucket name
	MinioObjectLocking      bool   // boolean (object locking)
	MinioFetchStat          bool
	ObjectSharing           bool   // boolean (object sharing)
	ObjectSharingExpiration string // expiration date
	ObjectSizeThreshold     string // object size
	PresignedUploadOnly     bool   // upload only via presigned urls

	// Main api (uspace) is using a manager/dispatcher/scheduler mechanism
	// configuration here.
	WssAddress              string
	WssLogsPath             string
	UspaceDispatcher        string
	UspaceJobQueueSize      string
	UspaceJobMaxWorkers     string
	UspaceJobExecutor       string
	UspaceJobMaxCPU         int64
	UspaceJobMaxMemory      int64
	UspaceJobMaxStorage     int64
	UspaceJobMaxParallelism int
	UspaceJobMaxTimeout     int64
	UspaceJobMaxLogicSize   int64
	// database storage of the jobs
	UspaceJobsDB             string
	UspaceJobsDBDriver       string
	UspaceJobsDBPath         string
	UspaceJobsDBMaxOpenConns string // maximum allowed simutanious open connections
	UspaceJobsDBMaxIdleConns string // maximum allowed idle connections
	UspaceJobsDBMaxLifetime  string // lifetime of an idle connection

	// ###################################
	// authentication (minioth),
	// 	- uses a storage handler rather than a storage system
	// ###################################
	MiniothAccessKey         string
	MiniothSecretKey         string
	MiniothDB                string // a path + name of the database
	MiniothDBPath            string
	MiniothDBDriver          string
	MiniothHandler           string // either database/db or plain/text
	MiniothAuditLogs         string
	MiniothAuditLogsMaxFetch int
}

// LoadConfig loads the config path to the environment and also creates and returns a ConfigStruct
func LoadConfig(path string) EnvConfig {
	if err := godotenv.Load(path); err != nil {
		log.Printf("Could not load %s config file. Using default variables", path)
	}

	split := strings.Split(path, "/")

	config := EnvConfig{
		ConfigPath: split[len(split)-1],
		Profile:    getEnv("PROFILE", "baremetal"),
		Verbose:    getBoolEnv("VERBOSE", "true"),
		AsOperator: getBoolEnv("AS_OPERATOR", "false"),
		Namespace:  getEnv("NAMESPACE", "default"),

		APIPort:          getEnv("API_PORT", "8079"),
		APIAddress:       getEnv("API_ADDRESS", "localhost"),
		APIUseTLS:        getBoolEnv("API_USE_TLS", "false"),
		APICertFile:      getEnv("API_CERT_FILE", "localhost.pem"),
		APIKeyFile:       getEnv("API_KEY_FILE", "localhost-key.pem"),
		APILogsPath:      getEnv("API_LOGS_PATH", "data/logs/jobs/job.log"),
		APILogsMaxFetch:  int(getInt64Env("API_LOGS_MAX_FETCH", 100)),
		APIGinMode:       getEnv("API_GIN_MODE", "debug"),
		IP:               getEnv("IP", "0.0.0.0"),
		FrontPort:        getEnv("FRONT_PORT", "8080"),
		FrontAddress:     getEnv("FRONT_ADDRESS", "localhost"),
		AuthPort:         getEnv("AUTH_PORT", "9090"),
		AuthAddress:      getEnv("AUTH_ADDRESS", "localhost"),
		AllowedOrigins:   getEnvs("ALLOWED_ORIGINS", []string{"None"}),
		AllowedHeaders:   getEnvs("ALLOWED_HEADERS", nil),
		AllowedMethods:   getEnvs("ALLOWED_METHODS", nil),
		Issuer:           getEnv("ISSUER", "http://localhost:9090"),
		JwtSecretKey:     getSecretKey("JWT_SECRET_KEY", true),
		JwtValidityHours: getFloatEnv("JWT_VALIDITY_HOURS", 1),
		ServiceSecretKey: getSecretKey("SERVICE_SECRET_KEY", true),
		Jwks:             getEnv("JWKS", "data/jwks/jwks.json"),

		StorageSystem:               getEnv("STORAGE_SYSTEM", "local"),
		LocalVolumesDefaultCapacity: getFloatEnv("LOCAL_VOLUMES_DEFAULT_CAPACITY", 20),
		LocalVolumesDefaultPath:     getEnv("LOCAL_VOLUMES_DEFAULT_PATH", "data/volumes/fslite"),

		FslServer:         getBoolEnv("FSL_SERVER", "true"),
		FslLocality:       getBoolEnv("FSL_LOCALITY", "true"),
		FslDB:             getEnv("DB_FSL", "database.db"),
		FslDBDriver:       getEnv("DB_FSL_DRIVER", "duckdb"),
		FslDBPath:         getEnv("DB_FSL_PATH", "data/db/fslite"),
		FslDBMaxOpenConns: getEnv("DB_FSL_MAX_OPEN_CONNS", "50"),
		FslDBMaxIdleConns: getEnv("DB_FSL_MAX_IDLE_CONNS", "10"),
		FslDBMaxLifetime:  getEnv("DB_FSL_MAX_LIFETIME", "10"),
		FslAccessKey:      getEnv("FSL_ACCESS_KEY", "fsladmin"),
		FslSecretKey:      getEnv("FSL_SECRET_KEY", "fsladmin"),
		FslUnlocked:       getBoolEnv("FSL_UNLOCKED", "false"),

		MinioEndpoint:           getEnv("MINIO_ENDPOINT", "minio:9000"),
		MinioNodeportEndpoint:   getEnv("MINIO_NODEPORT_ENDPOINT", "localhost:30101"),
		MinioAccessKey:          getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:          getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioUseSSL:             getEnv("MINIO_USE_SSL", "false"),
		MinioDefaultBucket:      getEnv("MINIO_DEFAULT_BUCKET", "default"),
		MinioObjectLocking:      getBoolEnv("MINIO_OBJECT_LOCKING", "false"),
		MinioFetchStat:          getBoolEnv("MINIO_FETCH_STAT", "false"),
		ObjectSharing:           getBoolEnv("OBJECT_SHARED", "false"),
		ObjectSharingExpiration: getEnv("OBJECT_SHARE_EXPIRE", "1440"),
		ObjectSizeThreshold:     getEnv("OBJECT_SIZE_THRESHOLD", "400000000"),
		PresignedUploadOnly:     getBoolEnv("ONLY_PRESIGNED_UPLOAD", "false"),

		UspaceDispatcher:         getEnv("J_DISPATCHER", "default"),
		UspaceJobExecutor:        getEnv("J_EXECUTOR", "docker"),
		UspaceJobQueueSize:       getEnv("J_QUEUE_SIZE", "100"),
		UspaceJobMaxWorkers:      getEnv("J_MAX_WORKERS", "10"),
		UspaceJobMaxCPU:          getInt64Env("J_MAX_CPU", 16),
		UspaceJobMaxMemory:       getInt64Env("J_MAX_MEM", 65000),
		UspaceJobMaxStorage:      getInt64Env("J_MAX_STORAGE", 20),
		UspaceJobMaxParallelism:  int(getInt64Env("J_MAX_PARALLELISM", 16)),
		UspaceJobMaxTimeout:      getInt64Env("J_MAX_TIMEOUT", 6000),
		UspaceJobMaxLogicSize:    getInt64Env("J_MAX_LOGIC_CHARS", 1000000),
		UspaceJobsDB:             getEnv("DB_JOBS", "jobs.db"),
		UspaceJobsDBDriver:       getEnv("DB_JOBS_DRIVER", "duckdb"),
		UspaceJobsDBPath:         getEnv("DB_JOBS_PATH", "data/db/uspace"),
		UspaceJobsDBMaxOpenConns: getEnv("DB_JOBS_MAX_OPEN_CONNS", "50"),
		UspaceJobsDBMaxIdleConns: getEnv("DB_JOBS_MAX_IDLE_CONNS", "10"),
		UspaceJobsDBMaxLifetime:  getEnv("DB_JOBS_MAX_LIFETIME", "10"),

		WssAddress:         getEnv("J_WS_ADDRESS", "localhost:8082"),
		WssLogsPath:        getEnv("J_WS_LOGS_PATH", "data/logs/jobs/job_ws.log"),
		WssAddressInternal: getEnv("WSS_ADDRESS_INTERNAL", "wss:8082"),

		MiniothAccessKey:         getEnv("MINIOTH_ACCESS_KEY", "root"),
		MiniothSecretKey:         getEnv("MINIOTH_SECRET_KEY", "root"),
		MiniothDB:                getEnv("MINIOTH_DB", "minioth.db"),
		MiniothDBPath:            getEnv("MINIOTH_DB_PATH", "data/db/minioth"),
		MiniothDBDriver:          getEnv("MINIOTH_DB_DRIVER", "duckdb"),
		MiniothHandler:           getEnv("MINIOTH_HANDLER", "database"),
		MiniothAuditLogs:         getEnv("MINIOTH_AUDIT_LOGS", "data/logs/minioth/audit.logs"),
		MiniothAuditLogsMaxFetch: int(getInt64Env("MINIOTH_AUDIT_LOGS_MAX_FETCH", 100)),
		HashCost:                 getEnv("HASH_COST", "4"),
	}

	log.Print(config.ToString())

	return config
}

func getSecretKey(envVar string, fail bool) []byte {
	secret := os.Getenv(envVar)
	if secret == "" {
		if fail {
			log.Fatalf("[CONF] Config variable %s must not be empty", envVar)
		}
		log.Printf("[CONF] config secret %s wasn't set", envVar)
	}

	return []byte(secret)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return fallback
}

func getInt64Env(key string, fallback int64) int64 {
	key = getEnv(key, "")
	keyInt, err := strconv.ParseInt(key, 10, 64)
	if err != nil {
		log.Printf("failed to parse int64 from var %v: %v\nfalling back to %v", key, err, fallback)

		return fallback
	}

	return keyInt
}

func getFloatEnv(key string, fallback float64) float64 {
	key = getEnv(key, "")
	keyFloat, err := strconv.ParseFloat(key, 64)
	if err != nil {
		log.Printf("failed to parse float64 from variable %v: %v\nfalling back... to %v", key, err, fallback)

		return fallback
	}

	return keyFloat
}

func getBoolEnv(key, fallback string) bool {
	key = getEnv(key, fallback)
	b, err := strconv.ParseBool(key)
	if err != nil {
		b, _ = strconv.ParseBool(fallback)
	}

	return b
}

func getEnvs(key string, fallback []string) []string {
	if value, exists := os.LookupEnv(key); exists {
		values := strings.SplitAfter(value, ",")

		return values
	}

	return fallback
}

// DeepCopy method with create an identicall deep copy of the given config
func (cfg *EnvConfig) DeepCopy() EnvConfig {
	// Copy all fields by value
	if cfg == nil {
		return EnvConfig{}
	}

	copied := EnvConfig{
		ConfigPath:                  cfg.ConfigPath,
		Issuer:                      cfg.Issuer,
		APIUseTLS:                   cfg.APIUseTLS,
		APICertFile:                 cfg.APICertFile,
		APIKeyFile:                  cfg.APIKeyFile,
		APIGinMode:                  cfg.APIGinMode,
		APILogsPath:                 cfg.APILogsPath,
		APILogsMaxFetch:             cfg.APILogsMaxFetch,
		APIPort:                     cfg.APIPort,
		APIAddress:                  cfg.APIAddress,
		FrontPort:                   cfg.FrontPort,
		FrontAddress:                cfg.FrontAddress,
		AuthPort:                    cfg.AuthPort,
		AuthAddress:                 cfg.AuthAddress,
		Jwks:                        cfg.Jwks,
		JwtValidityHours:            cfg.JwtValidityHours,
		HashCost:                    cfg.HashCost,
		AsOperator:                  cfg.AsOperator,
		Namespace:                   cfg.Namespace,
		StorageSystem:               cfg.StorageSystem,
		FslDB:                       cfg.FslDB,
		FslDBPath:                   cfg.FslDBPath,
		FslDBDriver:                 cfg.FslDBDriver,
		FslDBMaxOpenConns:           cfg.FslDBMaxOpenConns,
		FslDBMaxIdleConns:           cfg.FslDBMaxIdleConns,
		FslDBMaxLifetime:            cfg.FslDBMaxLifetime,
		FslAccessKey:                cfg.FslAccessKey,
		FslSecretKey:                cfg.FslSecretKey,
		FslServer:                   cfg.FslServer,
		FslLocality:                 cfg.FslLocality,
		LocalVolumesDefaultPath:     cfg.LocalVolumesDefaultPath,
		LocalVolumesDefaultCapacity: cfg.LocalVolumesDefaultCapacity,
		MinioAccessKey:              cfg.MinioAccessKey,
		MinioSecretKey:              cfg.MinioSecretKey,
		MinioEndpoint:               cfg.MinioEndpoint,
		MinioNodeportEndpoint:       cfg.MinioNodeportEndpoint,
		MinioUseSSL:                 cfg.MinioUseSSL,
		MinioDefaultBucket:          cfg.MinioDefaultBucket,
		MinioObjectLocking:          cfg.MinioObjectLocking,
		ObjectSharing:               cfg.ObjectSharing,
		ObjectSharingExpiration:     cfg.ObjectSharingExpiration,
		PresignedUploadOnly:         cfg.PresignedUploadOnly,
		ObjectSizeThreshold:         cfg.ObjectSizeThreshold,
		MinioFetchStat:              cfg.MinioFetchStat,
		UspaceDispatcher:            cfg.UspaceDispatcher,
		UspaceJobQueueSize:          cfg.UspaceJobQueueSize,
		UspaceJobMaxWorkers:         cfg.UspaceJobMaxWorkers,
		UspaceJobExecutor:           cfg.UspaceJobExecutor,
		WssAddress:                  cfg.WssAddress,
		WssAddressInternal:          cfg.WssAddressInternal,
		WssLogsPath:                 cfg.WssLogsPath,
		UspaceJobsDB:                cfg.UspaceJobsDB,
		UspaceJobsDBDriver:          cfg.UspaceJobsDBDriver,
		UspaceJobsDBPath:            cfg.UspaceJobsDBPath,
		UspaceJobsDBMaxOpenConns:    cfg.UspaceJobsDBMaxOpenConns,
		UspaceJobsDBMaxIdleConns:    cfg.UspaceJobsDBMaxIdleConns,
		UspaceJobsDBMaxLifetime:     cfg.UspaceJobsDBMaxLifetime,
		MiniothAccessKey:            cfg.MiniothAccessKey,
		MiniothSecretKey:            cfg.MiniothSecretKey,
		MiniothDB:                   cfg.MiniothDB,
		MiniothDBDriver:             cfg.MiniothDBDriver,
		MiniothHandler:              cfg.MiniothHandler,
		MiniothAuditLogs:            cfg.MiniothAuditLogs,
		MiniothAuditLogsMaxFetch:    cfg.MiniothAuditLogsMaxFetch,
	}

	if cfg.JwtSecretKey != nil {
		copied.JwtSecretKey = make([]byte, len(cfg.JwtSecretKey))
		copy(copied.JwtSecretKey, cfg.JwtSecretKey)
	}

	if cfg.ServiceSecretKey != nil {
		copied.ServiceSecretKey = make([]byte, len(cfg.ServiceSecretKey))
		copy(copied.ServiceSecretKey, cfg.ServiceSecretKey)
	}

	if cfg.AllowedOrigins != nil {
		copied.AllowedOrigins = make([]string, len(cfg.AllowedOrigins))
		copy(copied.AllowedOrigins, cfg.AllowedOrigins)
	}

	if cfg.AllowedHeaders != nil {
		copied.AllowedHeaders = make([]string, len(cfg.AllowedHeaders))
		copy(copied.AllowedHeaders, cfg.AllowedHeaders)
	}
	if cfg.AllowedMethods != nil {
		copied.AllowedMethods = make([]string, len(cfg.AllowedMethods))
		copy(copied.AllowedMethods, cfg.AllowedMethods)
	}

	return copied
}

// ToString method formats and returns its structure to a string
func (cfg *EnvConfig) ToString() string {
	var strBuilder strings.Builder

	reflectedValues := reflect.ValueOf(cfg).Elem()
	reflectedTypes := reflect.TypeOf(cfg).Elem()

	strBuilder.WriteString(fmt.Sprintf("[CFG]CONFIGURATION: %s\n", cfg.ConfigPath))

	for i := range reflectedValues.NumField() {
		fieldName := reflectedTypes.Field(i).Name
		fieldValue := reflectedValues.Field(i).Interface()

		if byteSlice, ok := fieldValue.([]byte); ok {
			fieldValue = string(byteSlice)
		}

		strBuilder.WriteString("[CFG]")
		if i < 9 {
			strBuilder.WriteString(fmt.Sprintf("%d.  ", i+1))
		} else {
			strBuilder.WriteString(fmt.Sprintf("%d. ", i+1))
		}
		if len(fieldName) <= 6 {
			strBuilder.WriteString(fmt.Sprintf("%v\t\t\t\t\t-> %v\n", fieldName, fieldValue))
		} else if len(fieldName) <= 14 {
			strBuilder.WriteString(fmt.Sprintf("%v\t\t\t\t-> %v\n", fieldName, fieldValue))
		} else if len(fieldName) <= 25 {
			strBuilder.WriteString(fmt.Sprintf("%v\t\t\t-> %v\n", fieldName, fieldValue))
		} else {
			strBuilder.WriteString(fmt.Sprintf("%v\t\t-> %v\n", fieldName, fieldValue))
		}
	}

	return strBuilder.String()
}

// Addr method returns the IP+Port of this config
func (cfg *EnvConfig) Addr(port string) string {
	return cfg.IP + ":" + port
}

// MakeConfig writes to a file the given config as a map
func MakeConfig(path string, fields any) error {
	jsonData, err := json.Marshal(fields)
	if err != nil {
		log.Printf("failed to marshal: %v", err)

		return err
	}

	cpth, err := os.Getwd()
	if err != nil {
		log.Printf("failed to get curpath: %v", err)

		return err
	}

	err = os.WriteFile(cpth+"/"+path, jsonData, os.ModePerm)
	if err != nil {
		log.Printf("failed to write config.json: %v", err)

		return err
	}

	return nil
}

// ReadConfig reads fields from a file and creates a string to string map
func ReadConfig(path string, secrets bool) (map[string]string, error) {
	cfg, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	cfgV := make(map[string]string)
	scanner := bufio.NewScanner(cfg)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		if !secrets && strings.Contains(strings.ToLower(line), "_key") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			err1 := cfg.Close()
			if err1 != nil {
				return nil, err1
			}

			return nil, err
		}
		cfgV[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	err = cfg.Close()
	if err != nil {
		return nil, err
	}

	return cfgV, nil
}
