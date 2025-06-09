// Package main includes the main definition
package main

import (
	"kyri56xcaesar/kuspace/internal/utils"
	wsr "kyri56xcaesar/kuspace/internal/wss"
)

func main() {
	cfg := utils.LoadConfig("configs/wss.conf")

	wsr.Serve(cfg)
}
