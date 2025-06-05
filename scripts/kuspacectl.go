// Package main
// kuspacectl is a deployment and management tool for the kuspace Kubernetes platform.
// It automates building, pushing, and deploying Docker images, as well as generating
// and applying Kubernetes manifests, ConfigMaps, and Secrets for the kuspace stack.
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
	deploymentsRoot = "deployments/kubernetes"

	uspaceConfPath   = "configs/uspace.conf"
	frontappConfPath = "configs/frontapp.conf"
	miniothConfPath  = "configs/minioth.conf"
	wssConfPath      = "configs/wss.conf"
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
		if err := run("docker", "buildx", "build", "-f", "build//Dockerfile.wss", "-t", "kyri56xcaesar/kuspace:wss-latest", "."); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "buildx", "build", "-f", "internal/uspace/applications/duckdb/Dockerfile.duck", "-t", "kyri56xcaesar/kuspace:applications-duckdb-v1", "internal/uspace/applications/duckdb"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "buildx", "build", "-f", "internal/uspace/applications/pypandas/Dockerfile.pypandas", "-t", "kyri56xcaesar/kuspace:applications-pypandas-v1", "internal/uspace/applications/pypandas"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "buildx", "build", "-f", "internal/uspace/applications/octave/Dockerfile.octave", "-t", "kyri56xcaesar/kuspace:applications-octave-v1", "internal/uspace/applications/octave"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "buildx", "build", "-f", "internal/uspace/applications/ffmpeg/Dockerfile.ffmpeg", "-t", "kyri56xcaesar/kuspace:applications-ffmpeg-v1", "internal/uspace/applications/ffmpeg"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "buildx", "build", "-f", "internal/uspace/applications/caengine/Dockerfile.caengine", "-t", "kyri56xcaesar/kuspace:applications-caengine-v1", "internal/uspace/applications/caengine"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "buildx", "build", "-f", "internal/uspace/applications/bash/Dockerfile.bash", "-t", "kyri56xcaesar/kuspace:applications-bash-v1", "internal/uspace/applications/bash"); err != nil {
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
		if err := run("docker", "push", "kyri56xcaesar/kuspace:wss-latest"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "push", "kyri56xcaesar/kuspace:applications-duckdb-v1"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "push", "kyri56xcaesar/kuspace:applications-pypandas-v1"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "push", "kyri56xcaesar/kuspace:applications-octave-v1"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "push", "kyri56xcaesar/kuspace:applications-ffmpeg-v1"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "push", "kyri56xcaesar/kuspace:applications-caengine-v1"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
		if err := run("docker", "push", "kyri56xcaesar/kuspace:applications-bash-v1"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Command failed: %v\n", err)
			os.Exit(1)
		}
	}

	// CREATE NAMESPACE
	{
		fmt.Println("🔧 Creating namespace:", ns)
		if err := run("kubectl", "create", "namespace", ns); err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to create namespace: %s :%v", ns, err)
			os.Exit(1)
		}
		err := os.Setenv("NAMESPACE", ns)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ failed to set namespace to environment: %v", err)
		}
	}

	// CREATE CONFIG MAPS & deploy
	{
		fmt.Println("📝 Transpiling configs to ConfigMaps")
		// parse minioth conf
		miniothConf, err := parseConfFile(miniothConfPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to parse configuration: %v", err)
			os.Exit(1)
		}
		mConfMap := buildConfigMapYAML("minioth", ns, miniothConf)

		err = os.WriteFile(deploymentsRoot+"/config-maps/minioth-config-map.yaml", []byte(mConfMap), 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to save confMap yaml: %v", err)
			os.Exit(1)
		}
		err = run("kubectl", "apply", "-f", deploymentsRoot+"/config-maps/minioth-config-map.yaml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to apply configMap yaml: %v", err)
			os.Exit(1)
		}

		// same for frontapp
		frontappConf, err := parseConfFile(frontappConfPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to parse configuration: %v", err)
			os.Exit(1)
		}
		fConfMap := buildConfigMapYAML("frontapp", ns, frontappConf)
		err = os.WriteFile(deploymentsRoot+"/config-maps/frontapp-config-map.yaml", []byte(fConfMap), 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to save confMap yaml: %v", err)
			os.Exit(1)
		}
		err = run("kubectl", "apply", "-f", deploymentsRoot+"/config-maps/frontapp-config-map.yaml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to apply configMap yaml: %v", err)
			os.Exit(1)
		}

		// same for uspace
		uspaceConf, err := parseConfFile(uspaceConfPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to parse configuration: %v", err)
			os.Exit(1)
		}
		uConfMap := buildConfigMapYAML("uspace", ns, uspaceConf)
		err = os.WriteFile(deploymentsRoot+"/config-maps/uspace-config-map.yaml", []byte(uConfMap), 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to save confMap yaml: %v", err)
			os.Exit(1)
		}
		err = run("kubectl", "apply", "-f", deploymentsRoot+"/config-maps/uspace-config-map.yaml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to apply configMap yaml: %v", err)
			os.Exit(1)
		}

		// use wss conf for wss
		wssConf, err := parseConfFile(wssConfPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to parse configuration: %v", err)
			os.Exit(1)
		}
		wsConfMap := buildConfigMapYAML("wss", ns, wssConf)
		err = os.WriteFile(deploymentsRoot+"/config-maps/wss-config-map.yaml", []byte(wsConfMap), 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to save confMap yaml: %v", err)
			os.Exit(1)
		}
		err = run("kubectl", "apply", "-f", deploymentsRoot+"/config-maps/wss-config-map.yaml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to apply configMap yaml: %v", err)
			os.Exit(1)
		}

	}

	// CREATE SECRETS & deploy
	{
		fmt.Println("🔑 Creating Secrets for JWT and inner service circle...")
		// parse 1 conf, each conf should have the same secret
		secrets, err := parseConfFileSecrets(uspaceConfPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to parse configuration: %v", err)
			os.Exit(1)
		}
		secretYaml := buildSecretsYAML(ns, secrets)
		err = os.WriteFile(deploymentsRoot+"/secrets/secrets.yaml", []byte(secretYaml), 0o600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to save secrets yaml: %v", err)
			os.Exit(1)
		}

		err = run("kubectl", "apply", "-f", deploymentsRoot+"/secrets/secrets.yaml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ failed to deploy secrets: %v", err)
			os.Exit(1)
		}
	}

	// DEPLOY PV and PVC
	{
		err := filepath.Walk(deploymentsRoot+"/pv", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".yaml") {
				fmt.Println("📦 Applying:", path)
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				yamlStr := string(content)
				yamlStr = strings.ReplaceAll(yamlStr, "${NAMESPACE}", ns)
				// save file:
				err = os.WriteFile(path, []byte(yamlStr), 0o644)
				if err != nil {
					fmt.Println("⚠️ failed to overwrite manifest")
				}
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

		err = filepath.Walk(deploymentsRoot+"/pvc", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".yaml") {
				fmt.Println("📦 Applying:", path)
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				yamlStr := string(content)
				yamlStr = strings.ReplaceAll(yamlStr, "${NAMESPACE}", ns)
				// save file:
				err = os.WriteFile(path, []byte(yamlStr), 0o644)
				if err != nil {
					fmt.Println("⚠️ failed to overwrite manifest")
				}
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

		fmt.Println("⏳ Giving the pv some time...")
		time.Sleep(time.Second * 3)
		err = run("kubectl", "wait", "--namespace", ns, "--for=condition=Available", "pvc/uspace-pv", "--timeout=30s")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed waiting for condition available of the pvc-uspace-pv: %v", err)
		}
		err = run("kubectl", "wait", "--namespace", ns, "--for=condition=Available", "pvc/minioth-pv", "--timeout=30s")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed waiting for condition available of the pvc-minioth-pv: %v", err)
		}
		err = run("kubectl", "wait", "--namespace", ns, "--for=condition=Available", "pvc/minio-pv", "--timeout=30s")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed waiting for condition available of the pvc-minio-pv: %v", err)
		}

	}
	// DEPLOY Services and Deployments
	// DEPLOY StatefulSets
	// DEPLOY MANIFESTS (ingress,rbac,storageclass)		 1 by 1 in the deployments directory
	{
		err := filepath.Walk(deploymentsRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".yaml") && !strings.Contains(path, "pv") {
				fmt.Println("📦 Applying:", path)
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}

				if strings.Contains(path, "system-ingress") {
					fmt.Println("⏳ Waiting for ingress controller deployment to be ready...")
					time.Sleep(time.Second * 6)
					err = run("kubectl", "wait", "--namespace", "ingress-nginx", "--for=condition=Ready", "pod", "-l", "app.kubernetes.io/component=controller", "--timeout=60s")
					if err != nil {
						fmt.Fprintf(os.Stderr, "⚠️ failed waiting for ingress to be ready: %v", err)
					}
				}

				yamlStr := string(content)
				yamlStr = strings.ReplaceAll(yamlStr, "${NAMESPACE}", ns)

				// save file:
				err = os.WriteFile(path, []byte(yamlStr), 0o644)
				if err != nil {
					fmt.Println("⚠️ failed to overwrite manifest")
				}

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
		index++
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
