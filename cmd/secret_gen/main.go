package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
)

var default_length int = 32

func main() {
	length := default_length
	if len(os.Args) == 2 {
		arg, err := strconv.Atoi(os.Args[1])
		if err == nil && (arg > 0 && arg <= 32) {
			length = arg
		}
	}

	tokenbytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, tokenbytes); err != nil {
		panic(err)
	}

	token := hex.EncodeToString(tokenbytes)
	fmt.Println("Service token: ", token)
}
