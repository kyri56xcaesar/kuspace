package main

import (
	"kyri56xcaesar/kuspace/internal/frontapp/ws"
	"log"
)

func main() {
	log.Fatal(ws.Serve())
}
