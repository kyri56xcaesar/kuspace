package main

import (
	"kyri56xcaesar/kuspace/internal/utils"
	wsr "kyri56xcaesar/kuspace/internal/ws_registry"
)

func main() {

	cfg := utils.LoadConfig("configs/wss.conf")
	wsr.Job_log_path = cfg.J_WS_LOGS_PATH
	wsr.Address = cfg.J_WS_ADDRESS

	wsr.Serve()

}
