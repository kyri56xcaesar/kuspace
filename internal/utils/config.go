package utils

/*
	Configuration of the kuspace services.

	Every setting is a field with an `env` tag (the variable, as it appears in
	configs/*.conf, the compose .env and the kubernetes config maps) and a
	`default`. One loader (loadEnv) fills them all; `secret` marks values that
	are never logged and `required` ones the services refuse to start without.

	The fields are grouped into sections by concern. EnvConfig embeds them
	all, so code reads cfg.JwtSecretKey (not cfg.Auth.JwtSecretKey), while a
	composite literal names the section: EnvConfig{AuthConfig: AuthConfig{...}}.
*/

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// EnvConfig is the whole configuration; each service reads the sections it
// needs.
type EnvConfig struct {
	ServiceConfig
	PeersConfig
	AuthConfig
	FsliteConfig
	MinioConfig
	JobsConfig
	MiniothConfig
}

// ServiceConfig is what every service has: identity, listener, logs, CORS.
type ServiceConfig struct {
	ConfigPath string // name of the .conf file (set by LoadConfig)
	Profile    string `env:"PROFILE" default:"baremetal"` // baremetal or container
	Verbose    bool   `env:"VERBOSE" default:"true"`
	AsOperator bool   `env:"AS_OPERATOR" default:"false"` // not used rn
	Namespace  string `env:"NAMESPACE" default:"default"` // kubernetes namespace

	IP              string `env:"IP" default:"0.0.0.0"` // listen address
	Issuer          string `env:"ISSUER" default:"http://localhost:9090"`
	APIUseTLS       bool   `env:"API_USE_TLS" default:"false"`
	APICertFile     string `env:"API_CERT_FILE" default:"localhost.pem"`
	APIKeyFile      string `env:"API_KEY_FILE" default:"localhost-key.pem"`
	APIGinMode      string `env:"API_GIN_MODE" default:"debug"`
	APILogsPath     string `env:"API_LOGS_PATH" default:"data/logs/jobs/job.log"`
	APILogsMaxFetch int    `env:"API_LOGS_MAX_FETCH" default:"100"` // max logs size (in MB)
	APIPort         string `env:"API_PORT" default:"8079"`
	APIAddress      string `env:"API_ADDRESS" default:"localhost"`

	AllowedOrigins []string `env:"ALLOWED_ORIGINS" default:"None"`
	AllowedHeaders []string `env:"ALLOWED_HEADERS"`
	AllowedMethods []string `env:"ALLOWED_METHODS"`
}

// PeersConfig locates the other services.
type PeersConfig struct {
	FrontPort          string `env:"FRONT_PORT" default:"8080"`
	FrontAddress       string `env:"FRONT_ADDRESS" default:"localhost"`
	AuthPort           string `env:"AUTH_PORT" default:"9090"` // minioth
	AuthAddress        string `env:"AUTH_ADDRESS" default:"localhost"`
	WssAddress         string `env:"J_WS_ADDRESS" default:"localhost:8082"`
	WssAddressInternal string `env:"WSS_ADDRESS_INTERNAL" default:"wss:8082"`
	WssLogsPath        string `env:"J_WS_LOGS_PATH" default:"data/logs/jobs/job_ws.log"`
}

// AuthConfig holds the token and service-to-service secrets.
type AuthConfig struct {
	JwtValidityHours float64 `env:"JWT_VALIDITY_HOURS" default:"1"`
	JwtSecretKey     []byte  `env:"JWT_SECRET_KEY" secret:"true" required:"true"`
	JwtSigningAlg    string  `env:"JWT_SIGNING_ALG" default:"HS256"` // HS256 (shared JwtSecretKey) or RS256 (minioth's JWKS)
	ServiceSecretKey []byte  `env:"SERVICE_SECRET_KEY" secret:"true" required:"true"`
	HashCost         string  `env:"HASH_COST" default:"4"` // bcrypt, used by minioth
}

// FsliteConfig configures fslite: uspace's metadata store, or the
// standalone fslite server.
type FsliteConfig struct {
	StorageSystem string `env:"STORAGE_SYSTEM" default:"local"` // minio, or local (fslite)

	FslDB             string `env:"FSL_DB" default:"database.db"`
	FslDBPath         string `env:"FSL_DB_PATH" default:"data/db/fslite"`
	FslDBDriver       string `env:"FSL_DB_DRIVER" default:"sqlite3"`
	FslDBMaxOpenConns string `env:"FSL_DB_MAX_OPEN_CONNS" default:"50"`
	FslDBMaxIdleConns string `env:"FSL_DB_MAX_IDLE_CONNS" default:"10"`
	FslDBMaxLifetime  string `env:"FSL_DB_MAX_LIFETIME" default:"10"`                // minutes
	FslAccessKey      string `env:"FSL_ACCESS_KEY" default:"fsladmin"`               // admin username
	FslSecretKey      string `env:"FSL_SECRET_KEY" default:"fsladmin" secret:"true"` // admin password
	FslServer         bool   `env:"FSL_SERVER" default:"true"`
	FslLocality       bool   `env:"FSL_LOCALITY" default:"true"`  // keep file contents on local disk
	FslUnlocked       bool   `env:"FSL_UNLOCKED" default:"false"` // don't limit or check usage/capacity

	LocalVolumesDefaultPath     string  `env:"LOCAL_VOLUMES_DEFAULT_PATH" default:"data/volumes/fslite"`
	LocalVolumesDefaultCapacity float64 `env:"LOCAL_VOLUMES_DEFAULT_CAPACITY" default:"20"` // GB
}

// MinioConfig configures the MinIO object store.
type MinioConfig struct {
	MinioNodeportEndpoint   string `env:"MINIO_NODEPORT_ENDPOINT" default:"localhost:30101"`
	MinioEndpoint           string `env:"MINIO_ENDPOINT" default:"minio:9000"`
	MinioAccessKey          string `env:"MINIO_ACCESS_KEY" default:"minioadmin"`
	MinioSecretKey          string `env:"MINIO_SECRET_KEY" default:"minioadmin" secret:"true"`
	MinioUseSSL             string `env:"MINIO_USE_SSL" default:"false"`
	MinioDefaultBucket      string `env:"MINIO_DEFAULT_BUCKET" default:"default"`
	MinioObjectLocking      bool   `env:"MINIO_OBJECT_LOCKING" default:"false"`
	MinioFetchStat          bool   `env:"MINIO_FETCH_STAT" default:"false"`
	ObjectSharing           bool   `env:"OBJECT_SHARED" default:"false"`
	ObjectSharingExpiration string `env:"OBJECT_SHARE_EXPIRE" default:"1440"`
	ObjectSizeThreshold     string `env:"OBJECT_SIZE_THRESHOLD" default:"400000000"`
	PresignedUploadOnly     bool   `env:"ONLY_PRESIGNED_UPLOAD" default:"false"`
}

// JobsConfig configures uspace's job queue, executors, limits and database.
type JobsConfig struct {
	UspaceDispatcher        string `env:"J_DISPATCHER" default:"default"`
	UspaceJobQueueSize      string `env:"J_QUEUE_SIZE" default:"100"`
	UspaceJobMaxWorkers     string `env:"J_MAX_WORKERS" default:"10"`
	UspaceJobExecutor       string `env:"J_EXECUTOR" default:"docker"`
	UspaceJobDockerNetwork  string `env:"J_DOCKER_NETWORK"` // docker executor: network job containers join (to reach MinIO)
	UspaceJobMaxCPU         int64  `env:"J_MAX_CPU" default:"16"`
	UspaceJobMaxMemory      int64  `env:"J_MAX_MEM" default:"65000"`
	UspaceJobMaxStorage     int64  `env:"J_MAX_STORAGE" default:"20"`
	UspaceJobMaxParallelism int    `env:"J_MAX_PARALLELISM" default:"16"`
	UspaceJobMaxTimeout     int64  `env:"J_MAX_TIMEOUT" default:"6000"`
	UspaceJobMaxLogicSize   int64  `env:"J_MAX_LOGIC_CHARS" default:"1000000"`
	UspaceJobTTL            int32  `env:"J_TTL" default:"3600"`

	UspaceJobsDB             string `env:"DB_JOBS" default:"jobs.db"`
	UspaceJobsDBDriver       string `env:"DB_JOBS_DRIVER" default:"sqlite3"`
	UspaceJobsDBPath         string `env:"DB_JOBS_PATH" default:"data/db/uspace"`
	UspaceJobsDBMaxOpenConns string `env:"DB_JOBS_MAX_OPEN_CONNS" default:"50"`
	UspaceJobsDBMaxIdleConns string `env:"DB_JOBS_MAX_IDLE_CONNS" default:"10"`
	UspaceJobsDBMaxLifetime  string `env:"DB_JOBS_MAX_LIFETIME" default:"10"` // minutes
}

// MiniothConfig is read by minioth (third_party/minioth has its own loader;
// these mirror it for the tools here).
type MiniothConfig struct {
	MiniothAccessKey         string `env:"MINIOTH_ACCESS_KEY" default:"root"`
	MiniothSecretKey         string `env:"MINIOTH_SECRET_KEY" default:"root" secret:"true"`
	MiniothDB                string `env:"MINIOTH_DB" default:"minioth.db"`
	MiniothDBPath            string `env:"MINIOTH_DB_PATH" default:"data/db/minioth"`
	MiniothDBDriver          string `env:"MINIOTH_DB_DRIVER" default:"sqlite3"`
	MiniothHandler           string `env:"MINIOTH_HANDLER" default:"database"` // database or plain (text files)
	MiniothAuditLogs         string `env:"MINIOTH_AUDIT_LOGS" default:"data/logs/minioth/audit.log"`
	MiniothAuditLogsMaxFetch int    `env:"MINIOTH_AUDIT_LOGS_MAX_FETCH" default:"100"`
}

// LoadConfig loads the .conf file at path into the environment (variables
// already set win) and builds the configuration from the environment. A
// missing required secret or an unparsable value stops the service.
func LoadConfig(path string) EnvConfig {
	if err := godotenv.Load(path); err != nil {
		log.Printf("Could not load %s config file. Using default variables", path)
	}

	var cfg EnvConfig
	if err := loadEnv(&cfg); err != nil {
		log.Fatalf("[CONF] invalid configuration:\n%v", err)
	}
	cfg.ConfigPath = filepath.Base(path)
	for _, name := range cfg.DefaultSecrets() {
		log.Printf("[CONF] WARNING: %s is set to its public default; set it in configs/secrets.env", name)
	}
	log.Print(cfg.ToString())

	return cfg
}

// loadEnv fills every `env`-tagged field of the struct dst points to (and
// of its embedded sections) from the environment, falling back to the
// field's `default`. It reports all problems at once.
func loadEnv(dst any) error {
	var errs []error
	walkConfig(reflect.ValueOf(dst).Elem(), func(f reflect.StructField, v reflect.Value) {
		name := f.Tag.Get("env")
		raw, set := os.LookupEnv(name)
		raw = strings.TrimSpace(raw)
		if !set || (raw == "" && v.Kind() != reflect.String) {
			raw = f.Tag.Get("default")
		}
		if raw == "" && f.Tag.Get("required") == "true" {
			errs = append(errs, fmt.Errorf("%s must be set", name))

			return
		}
		if err := setField(v, raw); err != nil {
			errs = append(errs, fmt.Errorf("%s=%q: %w", name, raw, err))
		}
	})

	return errors.Join(errs...)
}

// walkConfig calls fn for each `env`-tagged field, descending into
// embedded sections.
func walkConfig(v reflect.Value, fn func(reflect.StructField, reflect.Value)) {
	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		switch {
		case f.Anonymous && f.Type.Kind() == reflect.Struct:
			walkConfig(v.Field(i), fn)
		case f.Tag.Get("env") != "":
			fn(f, v.Field(i))
		}
	}
}

func setField(v reflect.Value, raw string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return errors.New("not a boolean")
		}
		v.SetBool(b)
	case reflect.Int, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, v.Type().Bits())
		if err != nil {
			return errors.New("not an integer")
		}
		v.SetInt(n)
	case reflect.Float64:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return errors.New("not a number")
		}
		v.SetFloat(n)
	case reflect.Slice:
		switch v.Type().Elem().Kind() {
		case reflect.Uint8: // []byte secret
			v.SetBytes([]byte(raw))
		case reflect.String: // comma separated list
			v.Set(reflect.ValueOf(splitList(raw)))
		default:
			return fmt.Errorf("unsupported config type %s", v.Type())
		}
	default:
		return fmt.Errorf("unsupported config type %s", v.Type())
	}

	return nil
}

// splitList splits "a, b,c" into [a b c] (nil for "").
func splitList(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}

	return out
}

// DefaultSecrets names the secrets that still hold their built-in (public)
// default value.
func (cfg *EnvConfig) DefaultSecrets() []string {
	var names []string
	walkConfig(reflect.ValueOf(cfg).Elem(), func(f reflect.StructField, v reflect.Value) {
		def := f.Tag.Get("default")
		if f.Tag.Get("secret") == "true" && def != "" && fieldString(v) == def {
			names = append(names, f.Tag.Get("env"))
		}
	})

	return names
}

func fieldString(v reflect.Value) string {
	switch x := v.Interface().(type) {
	case []byte:
		return string(x)
	case []string:
		return strings.Join(x, ",")
	default:
		return fmt.Sprint(x)
	}
}

// DeepCopy returns a copy of the configuration sharing no slices with it.
func (cfg *EnvConfig) DeepCopy() EnvConfig {
	if cfg == nil {
		return EnvConfig{}
	}
	c := *cfg
	c.JwtSecretKey = bytes.Clone(cfg.JwtSecretKey)
	c.ServiceSecretKey = bytes.Clone(cfg.ServiceSecretKey)
	c.AllowedOrigins = slices.Clone(cfg.AllowedOrigins)
	c.AllowedHeaders = slices.Clone(cfg.AllowedHeaders)
	c.AllowedMethods = slices.Clone(cfg.AllowedMethods)

	return c
}

// ToString formats the configuration for the startup log, secrets redacted.
func (cfg *EnvConfig) ToString() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[CFG]CONFIGURATION: %s\n", cfg.ConfigPath)
	walkConfig(reflect.ValueOf(cfg).Elem(), func(f reflect.StructField, v reflect.Value) {
		value := fieldString(v)
		if (f.Tag.Get("secret") == "true" || isSecretName(f.Name)) && value != "" {
			value = "<redacted>"
		}
		fmt.Fprintf(&b, "[CFG] %-30s = %s\n", f.Tag.Get("env"), value)
	})

	return b.String()
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

	err = os.WriteFile(cpth+"/"+path, jsonData, 0o600)
	if err != nil {
		log.Printf("failed to write config.json: %v", err)

		return err
	}

	return nil
}

// isSecretName reports whether a config key or EnvConfig field name holds a
// secret, so it is never logged or served (e.g. by /system-conf).
func isSecretName(name string) bool {
	n := strings.ToLower(name)

	return strings.Contains(n, "secret") || strings.Contains(n, "password") ||
		strings.HasSuffix(n, "_key")
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
		if !secrets && isSecretName(strings.SplitN(line, "=", 2)[0]) {
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
