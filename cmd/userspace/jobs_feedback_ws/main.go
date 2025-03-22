package main

import (
	"fmt"

	"github.com/gin-gonic/gin"

	wr "kyri56xcaesar/myThesis/internal/userspace/ws_registry"
	"kyri56xcaesar/myThesis/internal/utils"
)

func main() {

	cfg := utils.LoadWsConfig("configs/userspace.env")
	wr.Job_log_path = cfg.LOGS_PATH
	r := gin.Default()
	r.GET("/job-stream", wr.HandleJobWS)

	fmt.Println("Job WebSocket server running on ", cfg.WS_PORT)
	r.Run(cfg.WS_ADDRESS + ":" + cfg.WS_PORT)
}
