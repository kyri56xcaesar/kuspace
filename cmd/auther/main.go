package main

import (
	"kyri56xcaesar/myThesis/internal/auther"
)

func main() {
	srv := auther.NewService()
	srv.ServeHTTP()
}
