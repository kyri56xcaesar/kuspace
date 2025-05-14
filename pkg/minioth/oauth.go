package minioth

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// @Summary Healthcheck endpoint
// @Description Returns a basic status and version of the Minioth service.
// @Tags well-known
// @Produce json
// @Success 200 {object} map[string]string "Service status"
// @Router /.well-known/minioth [get]
func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": "0.0.1",
		"status":  "alive",
	})
}

// @Summary OpenID Connect Discovery Document
// @Description Provides OIDC configuration metadata for clients.
// @Tags well-known
// @Produce json
// @Success 200 {object} map[string]string "OIDC provider metadata"
// @Router /.well-known/openid-configuration [get]
func (srv *MService) openid_configuration(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"issuer":   srv.Minioth.Config.ISSUER,
		"jwks_uri": fmt.Sprintf("%s/.well-known/jwks.json", srv.Minioth.Config.ISSUER),
		// "authorization_endpoint":                fmt.Sprintf("%s/%s/login", srv.Config.ISSUER, VERSION),
		"token_endpoint":                        fmt.Sprintf("%s/%s/login", srv.Minioth.Config.ISSUER, VERSION),
		"userinfo_endpoint":                     fmt.Sprintf("%s/%s/user/me", srv.Minioth.Config.ISSUER, VERSION),
		"id_token_signing_alg_values_supported": "HS256",
	})
}

// @Summary JWKS endpoint
// @Description Returns the JSON Web Key Set used to verify JWTs.
// @Tags well-known
// @Produce json
// @Success 200 {object} map[string]any "JWKS keys"
// @Failure 500 {object} map[string]string "Failed to read or parse JWKS"
// @Router /.well-known/jwks.json [get]
func jwks_handler(c *gin.Context) {
	jwksFile, err := os.Open(jwksFilePath)
	if err != nil {
		log.Printf("failed to open jwks.json file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load JWKS"})
		return
	}
	defer jwksFile.Close()

	jwksData, err := io.ReadAll(jwksFile)
	if err != nil {
		log.Printf("failed to read jwks.json file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read JWKS"})
		return
	}

	var jwks map[string]any
	if err := json.Unmarshal(jwksData, &jwks); err != nil {
		log.Printf("failed to parse jwks.json file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse JWKS"})
		return
	}

	c.JSON(http.StatusOK, jwks)
}
