package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Username string `form:"username" json:"username" binding:"required,min=3,max=20"`
	Password string `form:"password" json:"password" binding:"required,min=4,max=100"`
}

type RegisterRequest struct {
	Username       string `form:"username" json:"username" binding:"required,min=3,max=20"`
	Password       string `form:"password" json:"password" binding:"required,min=4,max=100"`
	RepeatPassword string `form:"repeat-password" json:"repeat-password" binding:"required,min=4,max=100"`
	Email          string `form:"email" json:"email"`
}

type AuthServiceResponse struct {
	Token   string `json:"token"`
	Role    string `json:"role"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

func (srv *HTTPService) handleLogin(c *gin.Context) {
	log.Printf("%v request at %v from \n%v", c.Request.Method, c.Request.URL, c.Request.UserAgent())

	var login LoginRequest

	if err := c.ShouldBind(&login); err != nil {
		log.Printf("Login binding error: %v", err)
		// Respond with the appropriate error on the template.
		c.Redirect(http.StatusSeeOther, "/api/v1/login")
		return
	}

	// Forward login request to the auth service
	authServiceURL := "http://localhost:9090/v1/login"
	resp, err := forwardRequest(authServiceURL, login)
	if err != nil {
		log.Printf("Error forwarding login request: %v", err)
		c.Redirect(http.StatusSeeOther, "/api/v1/login")
		return

	}
	defer resp.Body.Close()

	// Check the response status from the auth service
	if resp.StatusCode != http.StatusOK {
		log.Printf("Auth service returned status: %v", resp.Status)
		c.Redirect(http.StatusSeeOther, "/api/v1/login")
		return
	}

	// Parse the response from the auth service
	var authResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Username     string `json:"username"`
		Groups       string `json:"groups"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
		log.Printf("Error decoding auth service response: %v", err)
		c.Redirect(http.StatusSeeOther, "/api/v1/login")
		return
	}

	log.Printf("response from auth: %+v", authResponse)

	c.SetCookie("username", authResponse.Username, 3600, "/api/v1/verified/", "", false, true) // Set the username cookie

	// Save tokens in cookies
	c.SetCookie("access_token", authResponse.AccessToken, 3600, "/api/v1/verified/", "", false, true)
	c.SetCookie("refresh_token", authResponse.RefreshToken, 3600, "/api/v1/verified/", "", false, true)

	// Redirect user based on their role
	if strings.Contains(authResponse.Groups, "admin") {
		c.Redirect(http.StatusSeeOther, "/api/v1/verified/admin-panel")
	} else {
		c.Redirect(http.StatusSeeOther, "/api/v1/verified/dashboard")
	}
}

func (srv *HTTPService) handleRegister(c *gin.Context) {
	log.Printf("%v request at %v from \n%v", c.Request.Method, c.Request.URL, c.Request.UserAgent())
}

func forwardRequest(destinationURI string, requestData interface{}) (*http.Response, error) {
	// Marshal the request data into JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("error marshalling request data: %w", err)
	}

	// Create a new POST request with the JSON data
	req, err := http.NewRequest(http.MethodPost, destinationURI, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Use an HTTP client to send the request
	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}
