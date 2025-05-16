package main

import (
	"kyri56xcaesar/kuspace/pkg/minioth"
)

func main() {
	m := minioth.NewMinioth("configs/minioth.conf")
	m.Service.ServeHTTP()
}
