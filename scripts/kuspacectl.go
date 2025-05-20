package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	deployments_root = "deployments/kubernetes"

	uspace_conf_path   = "configs/uspace.conf"
	frontapp_conf_path = "configs/frontapp.conf"
	minioth_conf_path  = "configs/minioth.conf"
)

var (
	namespace = flag.String("ns", "kuspace", "what namespace")
	destroy   = flag.Bool("destroy", false, "destroy the deployment")
	build     = flag.Bool("build", false, "build only")
	push      = flag.Bool("push", false, "push images to dockerhub?")
)

func main() {
	// PARSE ARGS
	flag.Parse()

	ns := *namespace
	des := *destroy
	b := *build
	p := *push
	//

	// DESTROY option
	if des {
		fmt.Println("🔥 Deleting namespace:", ns)
		if err := run("kubectl", "delete", "namespace", ns); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// BUILD images option
	if b {
		fmt.Println("🔧 Building all images")
		if err := run("docker", "buildx", "build", "-f", "build/Dockerfile.minioth", "-t", "kyri56xcaesar/kuspace:minioth-latest", "."); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "buildx", "build", "-f", "build/Dockerfile.uspace", "-t", "kyri56xcaesar/kuspace:uspace-latest", "."); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "buildx", "build", "-f", "build/Dockerfile.frontapp", "-t", "kyri56xcaesar/kuspace:frontapp-latest", "."); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "buildx", "build", "-f", "internal/uspace/applications/duckdb/Dockerfile.duck", "-t", "kyri56xcaesar/kuspace:applications-duckdb-v1", "internal/uspace/applications/duckdb"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
	}

	// PUSH images option
	if p {
		fmt.Println("🚀 Pushing images to Docker Hub!")
		if err := run("docker", "push", "kyri56xcaesar/kuspace:minioth-latest"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "push", "kyri56xcaesar/kuspace:uspace-latest"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "push", "kyri56xcaesar/kuspace:frontapp-latest"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "push", "kyri56xcaesar/kuspace:applications-duckdb-v1"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
	}

	// CREATE NAMESPACE
	{
		fmt.Println("🔧 Creating namespace:", ns)
		run("kubectl", "create", "namespace", ns)
		os.Setenv("NAMESPACE", ns)
	}

	// CREATE CONFIG MAPS & deploy
	{
		fmt.Println("📝 Transpiling configs to ConfigMaps")
		miniothConf, err := parseConfFile(minioth_conf_path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to parse configuration: %v", err)
		}
		mConfMap := buildConfigMapYAML("minioth", ns, miniothConf)

		err = os.WriteFile(deployments_root+"/minioth/minioth-config-map.yaml", []byte(mConfMap), 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to save confMap yaml: %v", err)
		}
		run("kubectl", "apply", "-f", deployments_root+"/minioth/minioth-config-map.yaml")

		frontappConf, err := parseConfFile(frontapp_conf_path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to parse configuration: %v", err)
		}
		fConfMap := buildConfigMapYAML("frontapp", ns, frontappConf)
		err = os.WriteFile(deployments_root+"/frontapp/frontapp-config-map.yaml", []byte(fConfMap), 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to save confMap yaml: %v", err)
		}
		run("kubectl", "apply", "-f", deployments_root+"/frontapp/frontapp-config-map.yaml")

		uspaceConf, err := parseConfFile(uspace_conf_path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to parse configuration: %v", err)
		}
		uConfMap := buildConfigMapYAML("uspace", ns, uspaceConf)
		err = os.WriteFile(deployments_root+"/uspace/uspace-config-map.yaml", []byte(uConfMap), 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to save confMap yaml: %v", err)
		}
		run("kubectl", "apply", "-f", deployments_root+"/uspace/uspace-config-map.yaml")
	}

	// CREATE SECRETS & deploy
	{
		fmt.Println("🔑 Creating Secrets for JWT and inner service circle...")
		// parse 1 conf, each conf should have the same secret
		secrets, err := parseConfFileSecrets(uspace_conf_path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to parse configuration: %v", err)
		}
		secretYaml := buildSecretsYAML(ns, secrets)
		err = os.WriteFile(deployments_root+"/secrets/secrets.yaml", []byte(secretYaml), 0o600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to save secrets yaml: %v", err)
		}

		err = run("kubectl", "apply", "-f", deployments_root+"/secrets/secrets.yaml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to deploy secrets: %v", err)
		}
	}

	// DEPLOY MANIFESTS 		 1 by 1 in the deployments directory
	{
		err := filepath.Walk(deployments_root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".yaml") {
				fmt.Println("📦 Applying:", path)
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}

				if strings.Contains(path, "system-ingress") {
					fmt.Println("⏳ Waiting for ingress controller deployment to be ready...")
					time.Sleep(time.Second * 6)
					run("kubectl", "wait", "--namespace", "ingress-nginx", "--for=condition=Ready", "pod", "-l", "app.kubernetes.io/component=controller", "--timeout=60s")
				}

				yamlStr := string(content)
				yamlStr = strings.ReplaceAll(yamlStr, "${NAMESPACE}", ns)

				apply := exec.Command("kubectl", "apply", "-f", "-")
				apply.Stdin = strings.NewReader(yamlStr)

				apply.Stdout = os.Stdout
				apply.Stderr = os.Stderr

				err = apply.Run()
				if err != nil {
					fmt.Fprintf(os.Stderr, "⚠️ Warning: failed to apply %s: %v\n", path, err)
				}
				return nil
			}
			return nil
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error applying manifests: %v\\n", err)
			os.Exit(1)
		}
	}

}

func run(cmd string, args ...string) error {
	fmt.Printf("🔹 Running: %s %s\n", cmd, strings.Join(args, " "))
	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func parseConfFile(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	data := make(map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.Contains(strings.ToLower(line), "secret") {
			continue // skip empty or comment lines or a secret
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			data[key] = val
		}
	}

	return data, nil
}

func parseConfFileSecrets(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	data := make(map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(strings.ToLower(line), "secret") {
			continue // skip empty or comment lines or non secret
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			data[key] = val
		}
	}

	return data, nil
}

func buildConfigMapYAML(name, namespace string, data map[string]string) string {
	var buf strings.Builder
	buf.WriteString("apiVersion: v1\n")
	buf.WriteString("kind: ConfigMap\n")
	buf.WriteString("metadata:\n")
	buf.WriteString(fmt.Sprintf("  name: %s-config\n", name))
	buf.WriteString(fmt.Sprintf("  namespace: %s\n", namespace))
	buf.WriteString("data:\n")
	buf.WriteString(fmt.Sprintf("  %s.conf: |\n", name))

	for k, v := range data {
		buf.WriteString(fmt.Sprintf("    %s=%s\n", k, v))
	}

	return buf.String()
}

func buildSecretsYAML(namespace string, data map[string]string) string {
	var (
		buf   strings.Builder
		index = 0
	)

	for key, value := range data {
		index += 1
		encodedValue := base64.StdEncoding.EncodeToString([]byte(value))

		buf.WriteString("apiVersion: v1\n")
		buf.WriteString("kind: Secret\n")
		buf.WriteString("metadata:\n")
		buf.WriteString(fmt.Sprintf("  name: %s\n", strings.ReplaceAll(strings.TrimSuffix(strings.ToLower(key), "_key"), "_", "-")))
		buf.WriteString(fmt.Sprintf("  namespace: %s\n", namespace))
		buf.WriteString("type: Opaque\n")
		buf.WriteString("data:\n")
		buf.WriteString(fmt.Sprintf("  %s: %s\n", key, encodedValue))

		if index < len(data) {
			buf.WriteString("---\n")
		}

	}

	return buf.String()
}
