package main

import (
	"kyri56xcaesar/myThesis/internal/userspace"
)

func main() {
	srv := userspace.NewUService("configs/userspace.env")
	srv.Serve()
}
