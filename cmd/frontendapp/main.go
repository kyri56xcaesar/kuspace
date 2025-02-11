package main

import (
	"kyri56xcaesar/myThesis/internal/frontendapp"
)

func main() {
	srv := frontendapp.NewService("configs/frontapp.env")
	srv.ServeHTTP()
}
