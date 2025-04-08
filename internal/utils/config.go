package utils

/*
* part of the utils module
*
* reusable code for extracting environmental variables.
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
	ConfigPath string
	CertFile   string
	KeyFile    string

	API_PORT             string
	API_ADDRESS          string
	API_J_SOCKET_ADDRESS string

	FRONT_PORT    string
	FRONT_ADDRESS string

	AUTH_PORT    string
	AUTH_ADDRESS string

	IP string

	DB_RV                string
	DB_RV_DRIVER         string
	DB_RV_Path           string
	DB_RV_MAX_OPEN_CONNS string
	DB_RV_MAX_IDLE_CONNS string
	DB_RV_MAX_LIFETIME   string

	J_DISPATCHER           string
	DB_JOBS                string
	DB_JOBS_DRIVER         string
	DB_JOBS_Path           string
	DB_JOBS_MAX_OPEN_CONNS string
	DB_JOBS_MAX_IDLE_CONNS string
	DB_JOBS_MAX_LIFETIME   string
	Volumes                string
	VCapacity              string

	JWTSecretKey  []byte
	JWTRefreshKey []byte

	ServiceSecret []byte

	AllowedOrigins []string
	AllowedHeaders []string
	AllowedMethods []string

	AS_OPERATOR bool
}

func LoadConfig(path string) EnvConfig {
	if err := godotenv.Load(path); err != nil {
		log.Printf("Could not load %s config file. Using default variables", path)
	}

	split := strings.Split(path, "/")

	config := EnvConfig{
		ConfigPath:             split[len(split)-1],
		CertFile:               getEnv("CERT", "localhost.pem"),
		KeyFile:                getEnv("KEY", "localhost-key.pem"),
		DB_RV:                  getEnv("DB_RV", "database.db"),
		DB_RV_DRIVER:           getEnv("DB_RV_DRIVER", "duckdb"),
		DB_RV_Path:             getEnv("DB_RV_PATH", "data/db"),
		DB_RV_MAX_OPEN_CONNS:   getEnv("DB_RV_MAX_OPEN_CONNS", "50"),
		DB_RV_MAX_IDLE_CONNS:   getEnv("DB_RV_MAX_IDLE_CONNS", "10"),
		DB_RV_MAX_LIFETIME:     getEnv("DB_RV_MAX_LIFETIME", "10"),
		J_DISPATCHER:           getEnv("J_DISPATCHER", "default"),
		DB_JOBS:                getEnv("DB_JOBS", "jobs.db"),
		DB_JOBS_DRIVER:         getEnv("DB_JOBS_DRIVER", "duckdb"),
		DB_JOBS_Path:           getEnv("DB_JOBS_PATH", "data/db"),
		DB_JOBS_MAX_OPEN_CONNS: getEnv("DB_JOBS_MAX_OPEN_CONNS", "50"),
		DB_JOBS_MAX_IDLE_CONNS: getEnv("DB_JOBS_MAX_IDLE_CONNS", "10"),
		DB_JOBS_MAX_LIFETIME:   getEnv("DB_JOBS_MAX_LIFETIME", "10"),
		Volumes:                getEnv("VOLUMES", "data/volumes"),
		VCapacity:              getEnv("V_CAPACITY", "20"),
		API_PORT:               getEnv("API_PORT", "8079"),
		API_ADDRESS:            getEnv("API_ADDRESS", "localhost"),
		API_J_SOCKET_ADDRESS:   getEnv("API_J_SOCKET_ADDRESS", "localhost:8082"),
		FRONT_PORT:             getEnv("FRONT_PORT", "8080"),
		FRONT_ADDRESS:          getEnv("FRONT_ADDRESS", "localhost"),
		AUTH_PORT:              getEnv("AUTH_PORT", "9090"),
		AUTH_ADDRESS:           getEnv("AUTH_ADDRESS", "localhost"),
		IP:                     getEnv("IP", "0.0.0.0"),
		AllowedOrigins:         getEnvs("ALLOWED_ORIGINS", []string{"None"}),
		AllowedHeaders:         getEnvs("ALLOWED_HEADERS", nil),
		AllowedMethods:         getEnvs("ALLOWED_METHODS", nil),
		JWTSecretKey:           getSecretKey("JWT_SECRET_KEY"),
		JWTRefreshKey:          getSecretKey("JWT_REFRESH_KEY"),
		ServiceSecret:          getSecretKey("SERVICE_SECRET"),
		AS_OPERATOR:            getBoolEnv("AS_OPERATOR", "false"),
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

func getBoolEnv(key, fallback string) bool {
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
		if len(fieldName) < 7 {
			strBuilder.WriteString(fmt.Sprintf("%v\t\t-> %v\n", fieldName, fieldValue))
		} else if len(fieldName) < 14 {
			strBuilder.WriteString(fmt.Sprintf("%v\t-> %v\n", fieldName, fieldValue))
		} else {
			strBuilder.WriteString(fmt.Sprintf("%v\t-> %v\n", fieldName, fieldValue))
		}
	}

	return strBuilder.String()
}

func (cfg *EnvConfig) Addr(port string) string {
	return cfg.IP + ":" + port
}

type WSConfig struct {
	WS_PORT    string
	WS_ADDRESS string
	LOGS_PATH  string
}

func LoadWsConfig(path string) WSConfig {
	if err := godotenv.Load(path); err != nil {
		log.Printf("Could not load %s config file. Using default variables", path)
	}

	return WSConfig{
		WS_PORT:    getEnv("WS_PORT", "8082"),
		WS_ADDRESS: getEnv("WS_ADDRESS", "localhost"),
		LOGS_PATH:  getEnv("LOGS_PATH", "data/logs/jobs/job.log"),
	}

}

func MakeConfig(path string, fields interface{}) error {

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
