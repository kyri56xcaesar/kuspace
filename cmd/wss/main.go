// Package main includes the main definition
package main

import (
	"kyri56xcaesar/kuspace/internal/config"
	wsr "kyri56xcaesar/kuspace/internal/wss"
)

func main() {
	cfg := config.LoadConfig("configs/wss.conf", config.Service, config.Peers, config.Auth)

	wsr.Serve(cfg)
}
