package frontendapp

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

var (
	googleConfig = &oauth2.Config{
		ClientID:     "GOOGLE_CLIENT_ID",     // Replace with your Google client ID
		ClientSecret: "GOOGLE_CLIENT_SECRET", // Replace with your Google client secret
		RedirectURL:  "http://localhost:8080/auth/google/callback",
		Scopes:       []string{"email", "profile"},
		Endpoint:     google.Endpoint,
	}

	githubConfig = &oauth2.Config{
		ClientID:     "GITHUB_CLIENT_ID",     // Replace with your GitHub client ID
		ClientSecret: "GITHUB_CLIENT_SECRET", // Replace with your GitHub client secret
		RedirectURL:  "http://localhost:8080/auth/github/callback",
		Scopes:       []string{"user"},
		Endpoint:     github.Endpoint,
	}
)

func googleAuthHandler(c *gin.Context) {
	url := googleConfig.AuthCodeURL("state", oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func googleCallbackHandler(c *gin.Context) {
	code := c.Query("code")
	token, err := googleConfig.Exchange(context.Background(), code)
	if err != nil {
		log.Println("Google token exchange error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Token exchange failed",
		})

		return
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo?accessToken="+token.AccessToken, nil)
	if err != nil {
		log.Printf("failed to create a new get request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create a new request"})

		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Google user info request error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "User info retrieval failed.",
		})

		return
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	var userInfo map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&userInfo)
	if err != nil {
		log.Printf("failed to decode to json: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to decode json"})

		return
	}
	c.JSON(http.StatusOK, userInfo)
}

func githubAuthHandler(c *gin.Context) {
	url := githubConfig.AuthCodeURL("state", oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func githubCallbackHandler(c *gin.Context) {
	code := c.Query("code")
	token, err := githubConfig.Exchange(context.Background(), code)
	if err != nil {
		log.Println("GitHub token exchange error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token exchange failed"})

		return
	}

	// Retrieve user info
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.github.com/user?accessToken="+token.AccessToken, nil)
	if err != nil {
		log.Printf("failed to create a new get request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create a new request"})

		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("GitHub user info request error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User info request failed"})

		return
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	var userInfo map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&userInfo)
	if err != nil {
		log.Printf("failed to decode to json: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to decode json"})

		return
	}
	c.JSON(http.StatusOK, userInfo)
}
