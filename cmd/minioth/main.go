package main

import (
	"github.com/kyri56xcaesar/minioth"
)

func main() {
	m := minioth.NewMinioth("root", true, "data/db/minioth/minioth.db")
	srv := minioth.NewMSerivce(&m, "configs/minioth.conf")
	srv.ServeHTTP()
}
