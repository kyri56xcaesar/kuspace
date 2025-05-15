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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type EnvConfig struct {
	ConfigPath string // path of the .conf file

	// ###################################
	// API CONFS
	// api as a general api (used for uspace)
	IP            string
	ISSUER        string
	API_USE_TLS   bool
	API_CERT_FILE string // path to a cert file
	API_KEY_FILE  string // path to a key file

	API_GIN_MODE       string
	API_LOGS_PATH      string // path to logs dir
	API_LOGS_MAX_FETCH int    // max logs size (in MB)
	API_PORT           string // main service port
	API_ADDRESS        string // main service address name (default IP)
	FRONT_PORT         string
	FRONT_ADDRESS      string
	AUTH_PORT          string
	AUTH_ADDRESS       string
	// front as in the frontend app  // frontend app itself should use api vars
	// auth as in an authentication app // if an auth app uses this configuration, it should reference api port as itself

	// service (main) authentication info
	JWKS               string // used by (minioth)
	JWT_VALIDITY_HOURS float64
	JWTSecretKey       []byte
	ServiceSecret      []byte
	AllowedOrigins     []string
	AllowedHeaders     []string
	AllowedMethods     []string
	HASH_COST          string // used by (minioth) //bcrypt

	AS_OPERATOR bool   // not used rn
	NAMESPACE   string // kubernetes namespace deployment

	// ###################################
	// storage/jobs
	// ###################################
	STORAGE_SYSTEM string // a storage system the service might use

	// 1. can use fslite
	// fslite conf
	DB_FSL                string // name of the database
	DB_FSL_PATH           string // path of the database
	DB_FSL_DRIVER         string // type of database driver, either duckdb or sqlite3
	DB_FSL_MAX_OPEN_CONNS string // maximum allowed simutanious open connections
	DB_FSL_MAX_IDLE_CONNS string // maximum allowed idle connections
	DB_FSL_MAX_LIFETIME   string // lifetime of an idle connection
	FSL_ACCESS_KEY        string // "root" or admin username for authentication
	FSL_SECRET_KEY        string // "root" or admin password for authentication
	FSL_SERVER            bool
	FSL_LOCALITY          bool

	// fslite uses a local data directory for storage
	LOCAL_VOLUMES_DEFAULT_PATH     string  // path to the data directory
	LOCAL_VOLUMES_DEFAULT_CAPACITY float64 // default capacity of the storage

	// 2. can use minio
	// minio conf
	MINIO_ACCESS_KEY      string // "root" or admin username for authentication
	MINIO_SECRET_KEY      string // "root" or admin password for authentication
	MINIO_ENDPOINT        string // minio API endpoint
	MINIO_USE_SSL         string // boolean (if https or not)
	MINIO_DEFAULT_BUCKET  string // default bucket name
	MINIO_OBJECT_LOCKING  bool   // boolean (object locking)
	OBJECT_SHARED         bool   // boolean (object sharing)
	OBJECT_SHARE_EXPIRE   string // expriation date
	ONLY_PRESIGNED_UPLOAD bool   // upload only via presigned urls
	OBJECT_SIZE_THRESHOLD string // object size
	MINIO_FETCH_STAT      bool

	// Main api (uspace) is using a manager/dispatcher/scheduler mechanism
	// configuration here.
	J_DISPATCHER   string
	J_QUEUE_SIZE   string
	J_MAX_WORKERS  string
	J_EXECUTOR     string
	J_WS_ADDRESS   string
	J_WS_LOGS_PATH string
	// database storage of the jobs
	DB_JOBS                string
	DB_JOBS_DRIVER         string
	DB_JOBS_PATH           string
	DB_JOBS_MAX_OPEN_CONNS string
	DB_JOBS_MAX_IDLE_CONNS string
	DB_JOBS_MAX_LIFETIME   string

	// ###################################
	// authentication (minioth),
	// 	- uses a storage handler rather than a storage system
	// ###################################
	MINIOTH_ACCESS_KEY           string
	MINIOTH_SECRET_KEY           string
	MINIOTH_DB                   string // a path + name of the database
	MINIOTH_DB_DRIVER            string
	MINIOTH_HANDLER              string // either database/db or plain/text
	MINIOTH_AUDIT_LOGS           string
	MINIOTH_AUDIT_LOGS_MAX_FETCH int
}

func LoadConfig(path string) EnvConfig {
	if err := godotenv.Load(path); err != nil {
		log.Printf("Could not load %s config file. Using default variables", path)
	}

	split := strings.Split(path, "/")

	config := EnvConfig{
		ConfigPath: split[len(split)-1],

		API_PORT:           getEnv("API_PORT", "8079"),
		API_ADDRESS:        getEnv("API_ADDRESS", "localhost"),
		API_USE_TLS:        getBoolEnv("API_USE_TLS", "false"),
		API_CERT_FILE:      getEnv("API_CERT_FILE", "localhost.pem"),
		API_KEY_FILE:       getEnv("API_KEY_FILE", "localhost-key.pem"),
		API_LOGS_PATH:      getEnv("API_LOGS_PATH", "data/logs/jobs/job.log"),
		API_LOGS_MAX_FETCH: int(getInt64Env("API_LOGS_MAX_FETCH", 100)),
		API_GIN_MODE:       getEnv("API_GIN_MODE", "debug"),
		STORAGE_SYSTEM:     getEnv("STORAGE_SYSTEM", "local"),

		FSL_SERVER:                     getBoolEnv("FSL_SERVER", "true"),
		FSL_LOCALITY:                   getBoolEnv("FSL_LOCALITY", "true"),
		DB_FSL:                         getEnv("DB_FSL", "database.db"),
		DB_FSL_DRIVER:                  getEnv("DB_FSL_DRIVER", "duckdb"),
		DB_FSL_PATH:                    getEnv("DB_FSL_PATH", "data/db/fslite"),
		DB_FSL_MAX_OPEN_CONNS:          getEnv("DB_FSL_MAX_OPEN_CONNS", "50"),
		DB_FSL_MAX_IDLE_CONNS:          getEnv("DB_FSL_MAX_IDLE_CONNS", "10"),
		DB_FSL_MAX_LIFETIME:            getEnv("DB_FSL_MAX_LIFETIME", "10"),
		FSL_ACCESS_KEY:                 getEnv("FSL_ACCESS_KEY", "fsladmin"),
		FSL_SECRET_KEY:                 getEnv("FSL_SECRET_KEY", "fsladmin"),
		LOCAL_VOLUMES_DEFAULT_CAPACITY: getFloatEnv("LOCAL_VOLUMES_DEFAULT_CAPACITY", 20),
		LOCAL_VOLUMES_DEFAULT_PATH:     getEnv("LOCAL_VOLUMES_DEFAULT_PATH", "data/volumes/fslite"),

		MINIO_ACCESS_KEY:      getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MINIO_SECRET_KEY:      getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MINIO_ENDPOINT:        getEnv("MINIO_ENDPOINT", "minio:9000"),
		MINIO_USE_SSL:         getEnv("MINIO_USE_SSL", "false"),
		MINIO_DEFAULT_BUCKET:  getEnv("MINIO_DEFAULT_BUCKET", "default"),
		MINIO_OBJECT_LOCKING:  getBoolEnv("MINIO_OBJECT_LOCKING", "false"),
		OBJECT_SHARED:         getBoolEnv("OBJECT_SHARED", "false"),
		OBJECT_SHARE_EXPIRE:   getEnv("OBJECT_SHARE_EXPIRE", "1440"),
		ONLY_PRESIGNED_UPLOAD: getBoolEnv("ONLY_PRESIGNED_UPLOAD", "false"),
		OBJECT_SIZE_THRESHOLD: getEnv("OBJECT_SIZE_THRESHOLD", "400000000"),
		MINIO_FETCH_STAT:      getBoolEnv("MINIO_FETCH_STAT", "false"),

		J_DISPATCHER:   getEnv("J_DISPATCHER", "default"),
		J_EXECUTOR:     getEnv("J_EXECUTOR", "docker"),
		J_QUEUE_SIZE:   getEnv("J_QUEUE_SIZE", "100"),
		J_MAX_WORKERS:  getEnv("J_MAX_WORKERS", "10"),
		J_WS_ADDRESS:   getEnv("J_WS_ADDRESS", "localhost:8082"),
		J_WS_LOGS_PATH: getEnv("J_WS_LOGS_PATH", "data/logs/jobs/job_ws.log"),

		DB_JOBS:                getEnv("DB_JOBS", "jobs.db"),
		DB_JOBS_DRIVER:         getEnv("DB_JOBS_DRIVER", "duckdb"),
		DB_JOBS_PATH:           getEnv("DB_JOBS_PATH", "data/db/uspace"),
		DB_JOBS_MAX_OPEN_CONNS: getEnv("DB_JOBS_MAX_OPEN_CONNS", "50"),
		DB_JOBS_MAX_IDLE_CONNS: getEnv("DB_JOBS_MAX_IDLE_CONNS", "10"),
		DB_JOBS_MAX_LIFETIME:   getEnv("DB_JOBS_MAX_LIFETIME", "10"),

		IP:                 getEnv("IP", "0.0.0.0"),
		FRONT_PORT:         getEnv("FRONT_PORT", "8080"),
		FRONT_ADDRESS:      getEnv("FRONT_ADDRESS", "localhost"),
		AUTH_PORT:          getEnv("AUTH_PORT", "9090"),
		AUTH_ADDRESS:       getEnv("AUTH_ADDRESS", "localhost"),
		AllowedOrigins:     getEnvs("ALLOWED_ORIGINS", []string{"None"}),
		AllowedHeaders:     getEnvs("ALLOWED_HEADERS", nil),
		AllowedMethods:     getEnvs("ALLOWED_METHODS", nil),
		ISSUER:             getEnv("ISSUER", "http://localhost:9090"),
		JWTSecretKey:       getSecretKey("JWT_SECRET_KEY"),
		JWT_VALIDITY_HOURS: getFloatEnv("JWT_VALIDITY_HOURS", 1),
		ServiceSecret:      getSecretKey("SERVICE_SECRET"),
		JWKS:               getEnv("JWKS", "data/jwks/jwks.json"),
		HASH_COST:          getEnv("HASH_COST", "4"),

		AS_OPERATOR: getBoolEnv("AS_OPERATOR", "false"),
		NAMESPACE:   getEnv("NAMESPACE", "default"),

		MINIOTH_ACCESS_KEY:           getEnv("MINIOTH_ACCESS_KEY", "root"),
		MINIOTH_SECRET_KEY:           getEnv("MINIOTH_SECRET_KEY", "root"),
		MINIOTH_DB:                   getEnv("MINIOTH_DB", "data/db/minioth/minioth.db"),
		MINIOTH_DB_DRIVER:            getEnv("MINIOTH_DB_DRIVER", "duckdb"),
		MINIOTH_HANDLER:              getEnv("MINIOTH_HANDLER", "database"),
		MINIOTH_AUDIT_LOGS:           getEnv("MINIOTH_AUDIT_LOGS", "data/logs/minioth/audit.logs"),
		MINIOTH_AUDIT_LOGS_MAX_FETCH: int(getInt64Env("MINIOTH_AUDIT_LOGS_MAX_FETCH", 100)),
	}

	log.Print(config.ToString())
	return config
}

func getSecretKey(envVar string) []byte {
	secret := os.Getenv(envVar)
	if secret == "" {
		log.Fatalf("Config variable %s must not be empty", envVar)
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
	key_int, err := strconv.ParseInt(key, 10, 64)
	if err != nil {
		log.Printf("failed to parse int64 from var %v: %v\nfalling back to %v", key, err, fallback)
		return fallback
	}
	return key_int
}

func getFloatEnv(key string, fallback float64) float64 {
	key = getEnv(key, "")
	key_float, err := strconv.ParseFloat(key, 64)
	if err != nil {
		log.Printf("failed to parse float64 from variable %v: %v\nfalling back... to %v", key, err, fallback)
		return fallback
	}
	return key_float
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

// CertFile string, KeyFile string, HTTPPort string, HTTPSPort string, IP string, DBfile string, AllowedOrigins []string, AllowedHeaders []string
// AllowedMethods []string
func (cfg *EnvConfig) ToString() string {
	var strBuilder strings.Builder

	reflectedValues := reflect.ValueOf(cfg).Elem()
	reflectedTypes := reflect.TypeOf(cfg).Elem()

	strBuilder.WriteString(fmt.Sprintf("[CFG]CONFIGURATION: %s\n", cfg.ConfigPath))

	for i := 0; i < reflectedValues.NumField(); i++ {
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
		if len(fieldName) < 6 {
			strBuilder.WriteString(fmt.Sprintf("%v\t\t\t\t-> %v\n", fieldName, fieldValue))
		} else if len(fieldName) <= 14 {
			strBuilder.WriteString(fmt.Sprintf("%v\t\t\t-> %v\n", fieldName, fieldValue))
		} else { //if len(fieldName) <= 20 {
			strBuilder.WriteString(fmt.Sprintf("%v\t\t-> %v\n", fieldName, fieldValue))
		}
	}

	return strBuilder.String()
}

func (cfg *EnvConfig) Addr(port string) string {
	return cfg.IP + ":" + port
}

func MakeConfig(path string, fields any) error {
	json_data, err := json.Marshal(fields)
	if err != nil {
		log.Printf("failed to marshal: %v", err)
		return err
	}

	cpth, err := os.Getwd()
	if err != nil {
		log.Printf("failed to get curpath: %v", err)
		return err
	}

	err = os.WriteFile(cpth+"/"+path, json_data, os.ModePerm)
	if err != nil {
		log.Printf("failed to write config.json: %v", err)
		return err
	}

	return nil
}
