package main

import (
	"log"
	"math/rand"
	"os"
	"strconv"
)

// func generateRandomString(length int) string {
// 	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
// 	b := make([]byte, length)
// 	for i := range b {
// 		b[i] = charset[rand.Intn(len(charset))]
// 	}
// 	return string(b)
// }

func writeRandomStringToFile(no_lines, line_length int, filename string) error {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
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
	var (
		output_path     = "tmp/"
		output          = "random_dataset.txt"
		amount_of_lines = 10
		line_length     = 20
		err             error
	)

	// argument 1, output_path,
	// argument 2, amount of lines
	// argument 3, length of each line
	args := os.Args
	switch len(args) {
	case 2:
		output = args[1]
	case 3:
		output = args[1]
		amount_of_lines, err = strconv.Atoi(args[2])
		if err != nil {
			log.Printf("failed to atoi #lines: %v", err)
			return
		}
	case 4:
		output = args[1]
		amount_of_lines, err = strconv.Atoi(args[2])
		if err != nil {
			log.Printf("failed to atoi #lines: %v", err)
			return
		}
		line_length, err = strconv.Atoi(args[3])
		if err != nil {
			log.Printf("failed to atoi line length value: %v", err)
			return
		}
	}

	log.Printf("useful args: %v", args[1:])

	err = writeRandomStringToFile(amount_of_lines, line_length, output_path+output)
	if err != nil {
		log.Println("error:", err)
	}
}
