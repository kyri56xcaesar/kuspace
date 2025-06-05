package testing

import (
	"encoding/json"
	"kyri56xcaesar/kuspace/pkg/minioth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	srv := minioth.NewMinioth("C:/Users/kyri/Documents/kuspace/tests/unit_tests/minioth/minioth_test.conf").Service
	srv.RegisterRoutes()
	return srv.Engine
}

func TestRegisterLoginPasswd(t *testing.T) {
	router := SetupRouter()

	body := `{"user":{"username":"testuser","password":{"hashpass":"testpass"}}}`
	req, _ := http.NewRequest("POST", "/v1/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Optionally check response content
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
	if _, ok := resp["message"]; !ok {
		t.Error("Expected message in response")
	}

	// perform a fail login
	body = `{"username":"asgasg","password":"asgsag"}`
	req, _ = http.NewRequest("POST", "/v1/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("Expected 400, got %d", w.Code)
	}

	// Optionally check response content

	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
	if _, ok := resp["error"]; !ok {
		t.Error("Expected error in response")
	}

	body = `{"username":"testuser","password":"testpass"}`
	req, _ = http.NewRequest("POST", "/v1/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Optionally check response content

	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
	if _, ok := resp["accessToken"]; !ok {
		t.Error("Expected accessToken in response")
	}

	// perform password change

}

func TestAdminUsersInDebugMode(t *testing.T) {
	router := SetupRouter()

	req, _ := http.NewRequest("GET", "/v1/admin/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestSmoke(t *testing.T) {
	router := SetupRouter()

	endpoints := []struct {
		method string
		url    string
		body   string
	}{
		{"POST", "/api/v1/register", `{"username":"a","password":"b"}`},
		{"POST", "/api/v1/login", `{"username":"a","password":"b"}`},
		{"GET", "/api/v1/user/token", ""},
		{"GET", "/api/v1/admin/users", ""}, // make sure debug mode is on
	}

	for _, ep := range endpoints {
		req, _ := http.NewRequest(ep.method, ep.url, strings.NewReader(ep.body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code >= 500 {
			t.Errorf("%s %s failed with status %d", ep.method, ep.url, w.Code)
		}
	}
}
