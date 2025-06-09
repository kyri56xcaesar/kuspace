// Package main has main app for this service
package main

import (
	"log"

	"kyri56xcaesar/kuspace/internal/frontapp/ws"
)

func main() {
	log.Fatal(ws.Serve())
}
