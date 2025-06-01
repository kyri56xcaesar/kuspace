package main

import (
	"kyri56xcaesar/kuspace/internal/utils"
	wsr "kyri56xcaesar/kuspace/internal/ws_registry"
)

func main() {

	cfg := utils.LoadConfig("configs/wss.conf")

	wsr.Serve(cfg)

}
