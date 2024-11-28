package api

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Username string `form:"username" json:"username" binding:"required,min=3,max=20"`
	Password string `form:"password" json:"password" binding:"required,min=4,max=100"`
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

var jwtSecret = []byte("mock_secret_key")

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

	// Delegete to Authentication Service
	// authServiceURL := srv.Config.AuthServiceURL + "/api/v1/login"
	//reqBody, err := json.Marshal(login)
	//if err != nil {
	//	log.Printf("Error marshaling auth request: %v", err)
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	//	// return
	//}

	//req, err := http.NewRequest("POST", authServiceURL, bytes.NewBuffer(reqBody))
	//if err != nil {
	//	log.Printf("Error creating request to auth service: %v", err)
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	//	// return
	//}

	// req.Header.Set("Content-Type", "application/json")

	//client := &http.Client{Timeout: 10 * time.Second}
	//resp, err := client.Do(req)
	//if err != nil {
	//	log.Printf("Error contacting auth service: %v", err)
	//	// c.HTML(http.StatusServiceUnavailable, "error.html", gin.H{"error": strconv.Itoa(http.StatusServiceUnavailable), "message": "Authentication service unavailable"},)
	//	// return
	//}
	//defer resp.Body.Close()

	//body, err := io.ReadAll(resp.Body)
	//if err != nil {
	//	log.Printf("Error reading auth service response: %v", err)
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	//	//return
	//}
	//var authServiceResp AuthServiceResponse
	//if err := json.Unmarshal(body, &authServiceResp); err != nil {
	//	log.Printf("Error unmarshaling auth service response: %v", err)
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response from authentication service"})
	//	//return
	//}
	//if resp.StatusCode != http.StatusOK {
	//	errorMessage := authServiceResp.Error
	//	if errorMessage == "" {
	//		errorMessage = "Authentication failed"
	//	}
	//	c.JSON(resp.StatusCode, gin.H{"error": errorMessage})
	//	return
	//}

	// NOTE: Assume authenticated.

	// Failure
	if login.Username != "admin" && login.Password != "admin" {
		log.Print("Authentication failed")
		c.HTML(http.StatusUnauthorized, "error.html", gin.H{"error": strconv.Itoa(http.StatusUnauthorized), "message": "Invalid credentials"})
		return
	}
	log.Print("Pseudo authentication succeeded")

	// Success
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Username: login.Username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
			IssuedAt:  time.Now().Unix(),
			Issuer:    "mock_app",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("Could not generate token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	c.Header("Authorization", "Bearer "+tokenString)
	c.SetCookie("auth_token", tokenString, 86400, "/api/v1/", "localhost", false, true)

	c.Redirect(http.StatusSeeOther, "/api/v1/verified/dashboard")
}

func (srv *HTTPService) handleDashboard(c *gin.Context) {
	cookie, err := c.Cookie("auth_token")
	if err != nil {
		log.Printf("error getting cookie: %v", err)
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error":   "cookie missing",
			"message": "required auth_token",
		})
		return
	}
	log.Printf("cookie: %v", cookie)
}
