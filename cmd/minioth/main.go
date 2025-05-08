package main

import (
	"kyri56xcaesar/myThesis/pkg/minioth"
)

func main() {
	m := minioth.NewMinioth("configs/minioth.conf")
	m.Service.ServeHTTP()
}
