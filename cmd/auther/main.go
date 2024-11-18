package main

import (
	"kyri56xcaesar/myThesis/internal/auther"
)

func main() {
	s := auther.HTTPService{}
	s.ServeHTTP()
}
