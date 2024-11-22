package auther

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

type AuthServiceRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20"`
	Password string `json:"password" binding:"required,min=6,max=100"`
}

type AuthServiceResponse struct {
	Token   string `json:"token"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

func (srv *HTTPService) handleLogin(c *gin.Context) {
	var authReq AuthServiceRequest

	if err := c.ShouldBind(&authReq); err != nil {
		log.Printf("Login binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	// Delegete to Authentication Service
	authServiceURL := srv.Config.AuthServiceURL + "/api/v1/login"
	authServiceReq := AuthServiceRequest{
		Username: authReq.Username,
		Password: authReq.Password,
	}

	reqBody, err := json.Marshal(authServiceReq)
	if err != nil {
		log.Printf("Error marshaling auth request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	req, err := http.NewRequest("POST", authServiceURL, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Error creating request to auth service: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error contacting auth service: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Authentication service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading auth service response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	var authServiceResp AuthServiceResponse
	if err := json.Unmarshal(body, &authServiceResp); err != nil {
		log.Printf("Error unmarshaling auth service response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response from authentication service"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		errorMessage := authServiceResp.Error
		if errorMessage == "" {
			errorMessage = "Authentication failed"
		}
		c.JSON(resp.StatusCode, gin.H{"error": errorMessage})
		return
	}
}

func (srv *HTTPService) handleDashboard(c *gin.Context) {
}
