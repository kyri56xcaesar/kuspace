package fslite_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"kyri56xcaesar/kuspace/internal/utils"
	"kyri56xcaesar/kuspace/pkg/fslite"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

var cfg utils.EnvConfig

func setupTestServer() *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg = utils.LoadConfig("tests/fslite/fslite_test.conf")
	fsl := fslite.NewFsLite(cfg)
	return fsl.Engine
}

func decodeJSONResponse(t *testing.T, body *bytes.Buffer) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(body.Bytes(), &result); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}
	return result
}

func TestRegister_ValidUser(t *testing.T) {
	router := setupTestServer()

	body := `{"username":"testuser1","password":"testpass1"}`
	req, _ := http.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}

	var result map[string]any
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	if _, ok := result["message"]; !ok {
		t.Error("Expected 'message' in response")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	router := setupTestServer()

	body := `{"username":"notexist","password":"badpass"}`
	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest && resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected 400 or 401, got %d", resp.Code)
	}
}

func TestLogin_ValidCredentials(t *testing.T) {
	router := setupTestServer()

	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, cfg.FslAccessKey, cfg.FslSecretKey)
	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200 or 401, got %d", resp.Code)
	}
}

func TestPasswd(t *testing.T) {

}

func TestUserMe(t *testing.T) {

}

func TestUserToken(t *testing.T) {

}

func TestRefreshToken(t *testing.T) {

}

func TestVerifyPassword_Admin(t *testing.T) {

}

func TestAdminGroups_DebugMode(t *testing.T) {

}

func TestAdminUseradd_DebugMode(t *testing.T) {

}

func TestAdminUserdel_DebugMode(t *testing.T) {

}

func TestAdminUserpatch_DebugMode(t *testing.T) {

}

func TestAdminUsermod_DebugMode(t *testing.T) {

}

func TestAdminGroupadd_DebugMode(t *testing.T) {

}

func TestAdminGrouppatch_DebugMode(t *testing.T) {

}

func TestAdminGroupmod_DebugMode(t *testing.T) {

}

func TestAdminGroupdel_DebugMode(t *testing.T) {

}
