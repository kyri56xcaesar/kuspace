// Package main has main app for this service
package main

import (
	frontendapp "kyri56xcaesar/kuspace/internal/frontapp"
)

func main() {
	srv := frontendapp.NewService("configs/frontapp.conf")
	srv.ServeHTTP()
}
