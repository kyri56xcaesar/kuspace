package main

import (
	"kyri56xcaesar/kuspace/internal/uspace"
)

func main() {
	srv := uspace.NewUService("configs/uspace.conf")
	srv.Serve()
}
