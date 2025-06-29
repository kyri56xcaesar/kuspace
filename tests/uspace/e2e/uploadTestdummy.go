package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	numFiles    = 500
	fileSize    = 1 * 1024 * 1024 // 1 MB
	uploadURL   = "http://localhost:30079/api/v1/resource/upload"
	tempDir     = "./tmp/upload_temp"
	concurrency = 50
	secret      = "1f4a96feb2603733a9e0fc6e9e79a7c6a94ae983b771212767f5793c418f2e30"
)

type result struct {
	latency time.Duration
	err     error
}

func main() {
	start := time.Now()

	// Step 1: Create temp directory
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		panic(err)
	}

	// Step 2: Generate dummy files
	fmt.Println("Generating dummy files...")
	files := generateDummyFiles(numFiles)

	// Step 3: Upload files with concurrency
	fmt.Println("Uploading files...")
	results := uploadFilesWithMetrics(files)

	// Step 4: Cleanup
	fmt.Println("Cleaning up files...")
	cleanupFiles(files)

	printStats(results, time.Since(start))
}

// Generate dummy files with random content
func generateDummyFiles(count int) []string {
	files := []string{}
	for i := 0; i < count; i++ {
		fileName := fmt.Sprintf("%s/file_%03d.txt", tempDir, i)
		err := createDummyFile(fileName, fileSize)
		if err != nil {
			panic(err)
		}
		files = append(files, fileName)
	}
	return files
}

// Create a dummy file with random content
func createDummyFile(filePath string, size int) error {
	data := make([]byte, size)
	_, err := rand.Read(data)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

func uploadFilesWithMetrics(files []string) []result {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	results := make([]result, len(files))

	for idx, file := range files {
		sem <- struct{}{}
		wg.Add(1)

		go func(i int, filePath string) {
			defer wg.Done()
			defer func() { <-sem }()

			start := time.Now()
			err := uploadFile(filePath)
			latency := time.Since(start)

			results[i] = result{
				latency: latency,
				err:     err,
			}
		}(idx, file)
	}

	wg.Wait()
	return results
}

// Upload a single file
func uploadFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("files", filepath.Base(file.Name()))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}

	_ = writer.Close()

	req, err := http.NewRequest("POST", uploadURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Service-Secret", secret)
	req.Header.Set("Access-Target", "0:test:/ 0:0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	fmt.Printf("✅ Uploaded %s - Status: %s - Response: %s\n",
		filepath.Base(filePath), resp.Status, string(respBody))

	return nil
}

// Delete all generated files
func cleanupFiles(files []string) {
	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			fmt.Printf("⚠️ Failed to delete %s: %v\n", file, err)
		}
	}
	_ = os.Remove(tempDir)
}

func printStats(results []result, totalDuration time.Duration) {
	var totalLatency time.Duration
	var maxLatency time.Duration
	latencies := []float64{}

	success := 0
	fail := 0

	for _, r := range results {
		if r.err == nil {
			success++
		} else {
			fail++
		}

		totalLatency += r.latency
		if r.latency > maxLatency {
			maxLatency = r.latency
		}
		latencies = append(latencies, float64(r.latency.Milliseconds()))
	}

	avgLatency := totalLatency / time.Duration(len(results))
	stdev := calcStdev(latencies)

	fmt.Println("----------- Benchmark Results -----------")
	fmt.Printf("Total Requests: %d\n", len(results))
	fmt.Printf("Concurrency: %d\n", concurrency)
	fmt.Printf("Successful Uploads: %d\n", success)
	fmt.Printf("Failed Uploads: %d\n", fail)
	fmt.Printf("Total Time: %s\n", totalDuration)
	fmt.Printf("Throughput: %.2f files/sec\n", float64(len(results))/totalDuration.Seconds())
	fmt.Printf("Average Latency: %s\n", avgLatency)
	fmt.Printf("Max Latency: %s\n", maxLatency)
	fmt.Printf("Latency Std Dev: %.2f ms\n", stdev)
	fmt.Println("-----------------------------------------")
}

func calcStdev(data []float64) float64 {
	var sum, mean, sqDiff float64
	n := float64(len(data))

	for _, v := range data {
		sum += v
	}
	mean = sum / n

	for _, v := range data {
		sqDiff += (v - mean) * (v - mean)
	}

	variance := sqDiff / n
	return math.Sqrt(variance)
}
