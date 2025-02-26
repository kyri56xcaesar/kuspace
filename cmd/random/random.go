package main

import (
	"fmt"
	"math/rand"
	"os"
)

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func writeRandomStringToFile(no_lines, line_length int, filename string) error {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for i := 0; i < no_lines; i++ {
		b := make([]byte, line_length)
		for i := range b {
			b[i] = charset[rand.Intn(len(charset))]
		}

		if _, err := f.WriteString(string(b) + "\n"); err != nil {
			return err
		}
	}

	return nil
}

func main() {

	err := writeRandomStringToFile(10, 20, "tmp/random.txt")
	if err != nil {
		fmt.Println("error:", err)
	}
}
