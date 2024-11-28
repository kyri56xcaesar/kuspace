package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Username string `form:"username" json:"username" binding:"required,min=3,max=20"`
	Password string `form:"password" json:"password" binding:"required,min=4,max=100"`
}

type AuthServiceResponse struct {
	Token   string `json:"token"`
	Role    string `json:"role"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

func (srv *HTTPService) handleLogin(c *gin.Context) {
	var login LoginRequest

	if err := c.ShouldBind(&login); err != nil {
		log.Printf("Login binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		// Respond with the appropriate error.
		// TODO:

		return
	}

	log.Print(login)

	// TODO: proper authentication linking

	// NOTE: Assume authenticated.
	authServiceResp := AuthServiceResponse{
		Token:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VybmFtZSI6ImtvdWxhcm9zIiwicm9sZSI6ImFkbWluIn0.Sa-KbdXFzqtsAI6urZEx58cssyg825FqsKmHU4T56pk",
		Role:    "admin",
		Message: "welcome admin",
		Error:   "",
	}

	c.SetCookie("auth_token", authServiceResp.Token, 86400, "/", "localhost", false, true)

	switch authServiceResp.Role {
	case "admin":
		c.Redirect(http.StatusSeeOther, "/api/v1/verified/admin-panel")
	case "user":
		c.Redirect(http.StatusSeeOther, "/api/v1/verified/dashboard")
	default:

	}
}
