package main

import (
	"fmt"

	"github.com/gin-gonic/gin"

	wr "kyri56xcaesar/kuspace/internal/uspace/ws_registry"
	"kyri56xcaesar/kuspace/internal/utils"
)

func main() {

	cfg := utils.LoadConfig("configs/userspace.env")
	wr.Job_log_path = cfg.J_WS_LOGS_PATH
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.GET("/job-stream", wr.HandleJobWS)
	r.DELETE("/delete-session", wr.HandleJobWSClose)

	fmt.Println("Job WebSocket server running on ", cfg.J_WS_ADDRESS)
	r.Run(cfg.J_WS_ADDRESS)
}
