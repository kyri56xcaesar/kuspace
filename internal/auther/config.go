package auther

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
)

type AConfig struct {
	ConfPath string
	IP       string
	API_PORT string

	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	ExposeHeaders  []string
	// and more...
	// ...
}

const (
	DEFAULT_conf_name string = "auther.config"
	DEFAULT_conf_path string = "configs/"
)

func (cfg *AConfig) setDefaults() {
	cfg.IP = "localhost"
	cfg.API_PORT = "8080"
	cfg.AllowedHeaders = []string{"*"}
	cfg.AllowedMethods = []string{"*"}
	cfg.AllowedOrigins = []string{"*"}
	cfg.ExposeHeaders = []string{"*"}
}

func (cfg *AConfig) Addr() string {
	return (cfg.IP + ":" + cfg.API_PORT)
}

func NewConfig() AConfig {
	cfg := AConfig{}
	cfg.setDefaults()

	return cfg
}

func (cfg *AConfig) LoadConfig(path string) error {
	var confFile *os.File
	var err error

	curpath, err := os.Getwd()
	cfg.ConfPath = curpath + path
	log.Printf("Current path: %v", curpath)
	confFile, err = os.Open(path)
	if err != nil {
		log.Printf("Given Path: %v, err: %v", path, err)
		confFile, err = os.Open(DEFAULT_conf_path + DEFAULT_conf_name)
		if err != nil {
			log.Print(err)
			return err
		}
		log.Print("Default config opened.")
	}
	defer confFile.Close()

	scanner := bufio.NewScanner(confFile)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "API_PORT":
			if value != "" {
				cfg.API_PORT = value
			}
		case "IP":
			if value != "" {
				cfg.IP = value
			}
		case "ORIGINS":
			if len(value) != 0 {
				cfg.AllowedOrigins = strings.Split(value, ",")
			}
		case "METHODS":
			if len(value) != 0 {
				cfg.AllowedMethods = strings.Split(value, ",")
			}
		case "HEADERS":
			if len(value) != 0 {
				cfg.AllowedHeaders = strings.Split(value, ",")
			}
		case "EXPOSE_HEADERS":
			if len(value) != 0 {
				cfg.ExposeHeaders = strings.Split(value, ",")
			}
		default:
		}
	}

	return nil
}

func (cfg *AConfig) toString() string {
	var strBuilder strings.Builder

	reflectedValues := reflect.ValueOf(cfg).Elem()
	reflectedTypes := reflect.TypeOf(cfg).Elem()

	strBuilder.WriteString(fmt.Sprintf("[CFG]CONFIGURATION: %s\n", cfg.ConfPath))

	for i := 0; i < reflectedValues.NumField(); i++ {
		fieldName := reflectedTypes.Field(i).Name
		fieldValue := reflectedValues.Field(i).Interface()

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
