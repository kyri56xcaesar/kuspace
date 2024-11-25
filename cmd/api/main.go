package main

import (
	"kyri56xcaesar/myThesis/internal/api"
)

func main() {
	srv := api.NewService()
	srv.ServeHTTP()
}
