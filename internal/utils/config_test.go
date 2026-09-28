package utils

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func TestLoadEnvDefaultsAndOverrides(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "jwt")
	t.Setenv("SERVICE_SECRET_KEY", "svc")
	t.Setenv("J_MAX_CPU", " 4 ") // conf values may carry trailing spaces
	t.Setenv("FSL_LOCALITY", "false")
	t.Setenv("ALLOWED_METHODS", "GET, POST,,PUT")
	t.Setenv("J_MAX_MEM", "") // set but empty: the default applies

	var cfg EnvConfig
	if err := loadEnv(&cfg); err != nil {
		t.Fatal(err)
	}
	if string(cfg.JwtSecretKey) != "jwt" || cfg.UspaceJobMaxCPU != 4 || cfg.FslLocality {
		t.Errorf("overrides not applied: jwt=%q cpu=%d locality=%v", cfg.JwtSecretKey, cfg.UspaceJobMaxCPU, cfg.FslLocality)
	}
	if cfg.UspaceJobMaxMemory != 65000 || cfg.APIPort != "8079" || cfg.UspaceJobTTL != 3600 || cfg.LocalVolumesDefaultCapacity != 20 {
		t.Errorf("defaults not applied: %+v", cfg.JobsConfig)
	}
	// used to be SplitAfter: ["GET,", " POST,", ...]
	if want := []string{"GET", "POST", "PUT"}; !slices.Equal(cfg.AllowedMethods, want) {
		t.Errorf("AllowedMethods = %q, want %q", cfg.AllowedMethods, want)
	}
}

func TestLoadEnvReportsEveryProblem(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "")
	t.Setenv("SERVICE_SECRET_KEY", "")
	t.Setenv("J_MAX_CPU", "lots")
	t.Setenv("FSL_SERVER", "maybe")

	var cfg EnvConfig
	err := loadEnv(&cfg)
	if err == nil {
		t.Fatal("bad configuration accepted")
	}
	for _, name := range []string{"JWT_SECRET_KEY", "SERVICE_SECRET_KEY", "J_MAX_CPU", "FSL_SERVER"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error does not mention %s: %v", name, err)
		}
	}
}

func TestEveryFieldIsLoadable(t *testing.T) {
	seen := map[string]string{}
	walkConfig(reflect.ValueOf(&EnvConfig{}).Elem(), func(f reflect.StructField, v reflect.Value) {
		name := f.Tag.Get("env")
		if prev, dup := seen[name]; dup {
			t.Errorf("%s is read by both %s and %s", name, prev, f.Name)
		}
		seen[name] = f.Name
		if def := f.Tag.Get("default"); def != "" {
			if err := setField(reflect.New(v.Type()).Elem(), def); err != nil {
				t.Errorf("%s: default %q does not parse: %v", name, def, err)
			}
		}
	})
}

func TestDeepCopySharesNothing(t *testing.T) {
	cfg := EnvConfig{
		AuthConfig:    AuthConfig{JwtSecretKey: []byte("k")},
		ServiceConfig: ServiceConfig{AllowedOrigins: []string{"a"}},
		JobsConfig:    JobsConfig{UspaceJobTTL: 7},
		FsliteConfig:  FsliteConfig{FslUnlocked: true}, // the hand-written copy dropped fields like this
	}
	c := cfg.DeepCopy()
	c.JwtSecretKey[0] = 'x'
	c.AllowedOrigins[0] = "b"
	if string(cfg.JwtSecretKey) != "k" || cfg.AllowedOrigins[0] != "a" {
		t.Error("copy shares slices with the original")
	}
	if c.UspaceJobTTL != 7 || !c.FslUnlocked {
		t.Errorf("copy lost fields: %+v", c)
	}
}

func TestToStringRedactsAndFlagsDefaults(t *testing.T) {
	var cfg EnvConfig
	t.Setenv("JWT_SECRET_KEY", "jwt-secret-value")
	t.Setenv("SERVICE_SECRET_KEY", "svc-secret-value")
	t.Setenv("MINIO_SECRET_KEY", "minio-secret-value")
	if err := loadEnv(&cfg); err != nil {
		t.Fatal(err)
	}
	out := cfg.ToString()
	for _, secret := range []string{"jwt-secret-value", "svc-secret-value", "minio-secret-value"} {
		if strings.Contains(out, secret) {
			t.Errorf("%s logged in clear", secret)
		}
	}
	if !strings.Contains(out, "FSL_SECRET_KEY                 = <redacted>") {
		t.Errorf("default secret not redacted:\n%s", out)
	}
	if !strings.Contains(out, "J_EXECUTOR") {
		t.Error("settings missing from the log")
	}
	defaults := cfg.DefaultSecrets()
	if !slices.Contains(defaults, "FSL_SECRET_KEY") || slices.Contains(defaults, "MINIO_SECRET_KEY") {
		t.Errorf("DefaultSecrets = %v", defaults)
	}
}

// Keys the Go services don't read, but other programs do.
var foreignKeys = map[string]bool{
	"JWT_REFRESH_SECRET_KEY":  true, // minioth
	"MINIOTH_SERVICE_SECRETS": true, // minioth
	"MINIO_ROOT_USER":         true, // the MinIO server
	"MINIO_ROOT_PASSWORD":     true,
}

// TestConfigFilesUseKnownKeys catches settings that silently do nothing:
// uspace.conf set DB_FSL_PATH for years while the loader read FSL_DB_PATH.
func TestConfigFilesUseKnownKeys(t *testing.T) {
	known := map[string]bool{}
	walkConfig(reflect.ValueOf(&EnvConfig{}).Elem(), func(f reflect.StructField, _ reflect.Value) {
		known[f.Tag.Get("env")] = true
	})
	root := filepath.Join("..", "..")
	files := []string{"configs/uspace.conf", "configs/frontapp.conf", "configs/wss.conf", "configs/fslite.conf",
		"configs/secrets.env.example",
		"deployments/kubernetes/config-maps/uspace-config-map.yaml",
		"deployments/kubernetes/config-maps/frontapp-config-map.yaml",
		"deployments/kubernetes/config-maps/wss-config-map.yaml"}
	for _, name := range files {
		f, err := os.Open(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		for n := 1; sc.Scan(); n++ {
			key, _, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
			if !ok || strings.HasPrefix(key, "#") || strings.ContainsAny(key, " :") {
				continue
			}
			if !known[key] && !foreignKeys[key] {
				t.Errorf("%s:%d: %s is not read by any service", name, n, key)
			}
		}
		_ = f.Close()
	}
}

// The loader is strict now (a bad number stops the service instead of
// silently using the default), so every shipped conf file must load.
func TestShippedConfigsLoad(t *testing.T) {
	for _, name := range []string{"uspace.conf", "frontapp.conf", "wss.conf", "fslite.conf"} {
		t.Run(name, func(t *testing.T) {
			values, err := godotenv.Read(filepath.Join("..", "..", "configs", name))
			if err != nil {
				t.Fatal(err)
			}
			for k, v := range values {
				t.Setenv(k, v)
			}
			t.Setenv("JWT_SECRET_KEY", "x")
			t.Setenv("SERVICE_SECRET_KEY", "x")
			var cfg EnvConfig
			if err := loadEnv(&cfg); err != nil {
				t.Errorf("%s does not load:\n%v", name, err)
			}
		})
	}
}
