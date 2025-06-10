package minioth_test

import (
	"bytes"
	"encoding/json"
	"kyri56xcaesar/kuspace/pkg/minioth"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestServer() *gin.Engine {
	gin.SetMode(gin.TestMode)
	srv := minioth.NewMinioth("tests/minioth/minioth_test.conf").Service
	srv.RegisterRoutes()
	return srv.Engine
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

	body := `{"user":{"username":"testuser1","password":{"hashpass":"testpass1"}}}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/register", bytes.NewBufferString(body))
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
	req, _ := http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest && resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected 400 or 401, got %d", resp.Code)
	}
}

func TestAdminUsers_DebugMode(t *testing.T) {
	router := setupTestServer()

	req, _ := http.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", "e122ea7e")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}

	req, _ = http.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected 401, got %d", resp.Code)
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
