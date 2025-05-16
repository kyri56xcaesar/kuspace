package main

import (
	"kyri56xcaesar/kuspace/internal/frontendapp"
)

func main() {
	srv := frontendapp.NewService("configs/frontapp.conf")
	srv.ServeHTTP()
}
