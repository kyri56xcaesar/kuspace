package minioth_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"kyri56xcaesar/kuspace/internal/utils"
	"kyri56xcaesar/kuspace/pkg/minioth"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
)

var (
	access, pass = "root", "root" // put ur conf creds
	serviceSec   = "e122ea7e"     // put ur service secret key
)

func setupTestServer() *gin.Engine {
	gin.SetMode(gin.TestMode)
	cwd, err := os.Getwd()
	if err != nil {
		panic("failed to retrieve working directory")
	}

	fmt.Printf("working directory: %v", cwd)
	srv := minioth.NewMinioth(fmt.Sprintf("%s/minioth_test.conf", cwd)).Service
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

	username, err := utils.GenerateRandomString(5)
	if err != nil {
		panic(err)
	}

	body := fmt.Sprintf(`{"user":{"username":"%s", "info":"test@mail.com",
		"password":{"hashpass":"Testpass1"}}}`, username)
	req, _ := http.NewRequest(http.MethodPost, "/v1/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}

	var result map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
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

	if resp.Code != http.StatusBadRequest && resp.Code != http.StatusUnauthorized && resp.Code != http.StatusNotFound {
		t.Errorf("Expected 400 or 401 or 404, got %d", resp.Code)
	}
}

func TestLogin_ValidDifferentTokens(t *testing.T) {
	router := setupTestServer()

	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, access, pass)
	req, _ := http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Signing-Alg", "HS256")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200 , got %d", resp.Code)
	}
	var responseBody struct {
		User  utils.User `json:"user"`
		Token string     `json:"accessToken"`
	}

	err := json.Unmarshal(resp.Body.Bytes(), &responseBody)
	if err != nil {
		t.Errorf("failed to unmarshal response body: %v", err)
	}

	fmt.Fprintf(os.Stderr, "resp: %+v", responseBody)
	if responseBody.Token == "" {
		t.Error("token was empty")
	}

	req, _ = http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Signing-Alg", "RS256")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}

	var responseBody2 struct {
		Token string     `json:"accessToken"`
		User  utils.User `json:"user"`
	}

	err = json.Unmarshal(resp.Body.Bytes(), &responseBody2)
	if err != nil {
		t.Errorf("failed to unmarshal response body: %v", err)
	}

	fmt.Fprintf(os.Stderr, "resp: %+v", responseBody)
	if responseBody.Token == "" {
		t.Error("token was empty")
	}
}

func TestPasswd(t *testing.T) {
	router := setupTestServer()
	// register a random user
	// random username
	username, err := utils.GenerateRandomString(5)
	if err != nil {
		panic(err)
	}
	body := fmt.Sprintf(`{"user":{"username":"%s", "info":"test@mail.com","password":{"hashpass":"Testpass1"}}}`, username)
	req0, _ := http.NewRequest(http.MethodPost, "/v1/register", bytes.NewBufferString(body))
	req0.Header.Set("Content-Type", "application/json")

	resp0 := httptest.NewRecorder()
	router.ServeHTTP(resp0, req0)

	if resp0.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp0.Code)
	}

	var result0 map[string]any
	err = json.Unmarshal(resp0.Body.Bytes(), &result0)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	if _, ok := result0["message"]; !ok {
		t.Error("Expected 'message' in response")
	}

	// login as root
	body = fmt.Sprintf(`{"username":"%s","password":"%s"}`, access, pass)
	req, _ := http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}
	var result map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	if _, ok := result["accessToken"]; !ok {
		t.Error("Expected 'message' in response")
	}
	adminToken := result["accessToken"].(string)

	// login as new user
	body = fmt.Sprintf(`{"username":"%s","password":"Testpass1"}`, username)
	req2, _ := http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp2.Code)
	}

	var result2 map[string]any
	err = json.Unmarshal(resp2.Body.Bytes(), &result2)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	if _, ok := result2["accessToken"]; !ok {
		t.Error("Expected 'message' in response")
	}
	userToken := result2["accessToken"].(string)

	// change password as admin
	body = fmt.Sprintf(`{"username":"%s","password":"Testpass2"}`, username)
	req3, _ := http.NewRequest(http.MethodPost, "/v1/passwd", bytes.NewBufferString(body))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", fmt.Sprintf("Bearer %s", adminToken))
	resp3 := httptest.NewRecorder()
	router.ServeHTTP(resp3, req3)
	if resp3.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp3.Code)
	}
	var result3 map[string]any
	err = json.Unmarshal(resp3.Body.Bytes(), &result3)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	fmt.Printf("result body: %+v", result3)

	// attemptlogin again
	body = fmt.Sprintf(`{"username":"%s","password":"Testpass2"}`, username)
	req, _ = http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp4 := httptest.NewRecorder()
	router.ServeHTTP(resp4, req)
	if resp4.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp4.Code)
	}
	var result4 map[string]any
	err = json.Unmarshal(resp4.Body.Bytes(), &result4)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	fmt.Printf("result body: %+v", result4)

	// attempt fail login
	body = fmt.Sprintf(`{"username":"%s","password":"Testpass1"}`, username)
	req, _ = http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp5 := httptest.NewRecorder()
	router.ServeHTTP(resp5, req)
	if resp5.Code != http.StatusUnauthorized && resp5.Code != http.StatusForbidden {
		t.Errorf("Expected 403 or 401, got %d", resp5.Code)
	}
	var result5 map[string]any
	err = json.Unmarshal(resp5.Body.Bytes(), &result5)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	// fmt.Printf("result body: %+v", result5)

	// change password as user
	body = fmt.Sprintf(`{"username":"%s","password":"Testpass3"}`, username)
	req, _ = http.NewRequest(http.MethodPost, "/v1/passwd", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", userToken))
	resp6 := httptest.NewRecorder()

	router.ServeHTTP(resp6, req)
	if resp6.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp6.Code)
	}
	var result6 map[string]any
	err = json.Unmarshal(resp6.Body.Bytes(), &result6)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	// fmt.Printf("result body: %+v", result6)

	// attempt login again
	body = fmt.Sprintf(`{"username":"%s","password":"Testpass3"}`, username)
	req, _ = http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp7 := httptest.NewRecorder()

	router.ServeHTTP(resp7, req)
	if resp7.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp7.Code)
	}
	var result7 map[string]any
	err = json.Unmarshal(resp7.Body.Bytes(), &result7)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	// fmt.Printf("result body: %+v", result7)
}

func TestUserMe(t *testing.T) {
	router := setupTestServer()

	// login as root
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, access, pass)
	req, _ := http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
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
	if _, ok := result["accessToken"]; !ok {
		t.Error("Expected 'message' in response")
	}
	adminToken := result["accessToken"].(string)

	// check
	req, _ = http.NewRequest(http.MethodGet, "/v1/user/me", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", adminToken))
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req)
	if resp2.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp2.Code)
	}

	var res []map[string]any
	err = json.Unmarshal(resp2.Body.Bytes(), &res)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	fmt.Printf("response: %+v", res)

}

func TestUserToken(t *testing.T) {
	router := setupTestServer()

	// login as root
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, access, pass)
	req, _ := http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
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
	if _, ok := result["accessToken"]; !ok {
		t.Error("Expected 'message' in response")
	}
	adminToken := result["accessToken"].(string)

	// check
	req, _ = http.NewRequest(http.MethodGet, "/v1/user/token", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", adminToken))
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req)
	if resp2.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp2.Code)
	}

	var res map[string]any
	err = json.Unmarshal(resp2.Body.Bytes(), &res)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	fmt.Printf("response: %+v", res)

}

func TestRefreshToken(t *testing.T) {
	router := setupTestServer()
	// login
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, access, pass)
	req, _ := http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Signing-Alg", "HS256")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200 , got %d", resp.Code)
	}
	var result map[string]any
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	if _, ok := result["accessToken"]; !ok {
		t.Error("Expected 'message' in response")
	}
	adminToken := result["accessToken"].(string)

	//refresh token
	body = fmt.Sprintf(`{"accessToken":"%s"}`, adminToken)
	req, _ = http.NewRequest(http.MethodPost, "/v1/user/refresh-token", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req)
	if resp2.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp2.Code)
	}

	var res map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &res)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	if _, ok := res["accessToken"]; !ok {
		t.Error("Expected 'accessToken' in response")
	}
}

func TestVerifyPassword_Admin(t *testing.T) {
	router := setupTestServer()
	// login
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, access, pass)
	req, _ := http.NewRequest(http.MethodPost, "/v1/admin/verify-password", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", serviceSec)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200 , got %d", resp.Code)
	}
	var result map[string]any
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	if _, ok := result["status"]; !ok {
		t.Error("Expected 'status' in response")
	}
}

func TestAdminUsers_DebugMode(t *testing.T) {
	router := setupTestServer()

	req, _ := http.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", serviceSec)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}

	req, _ = http.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	fmt.Printf("statuscode: %v equals?: %v status forbidden?: %v", resp.Code, resp.Code == http.StatusForbidden, http.StatusForbidden)
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}

func TestAdminGroups_DebugMode(t *testing.T) {
	router := setupTestServer()

	req, _ := http.NewRequest(http.MethodGet, "/v1/admin/groups", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", serviceSec)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}

	req, _ = http.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}

func TestAdminUseradd_DebugMode(t *testing.T) {
	router := setupTestServer()

	username, err := utils.GenerateRandomString(5)
	if err != nil {
		panic(err)
	}

	body := fmt.Sprintf(`{"user":{"username":"%s", "info":"test@mail.com",
		"password":{"hashpass":"Testpass1"}}}`, username)
	req, _ := http.NewRequest(http.MethodPost, "/v1/admin/useradd", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", serviceSec)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}

	var result map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	if _, ok := result["message"]; !ok {
		t.Error("Expected 'message' in response")
	}
}

func TestAdminUserdel_DebugMode(t *testing.T) {
	router := setupTestServer()

	username, err := utils.GenerateRandomString(5)
	if err != nil {
		panic(err)
	}

	// add a new user, then delete him
	body := fmt.Sprintf(`{"user":{"username":"%s", "info":"test@mail.com",
		"password":{"hashpass":"Testpass1"}}}`, username)
	req, _ := http.NewRequest(http.MethodPost, "/v1/admin/useradd", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", serviceSec)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}

	var result map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	if _, ok := result["message"]; !ok {
		t.Error("Expected 'message' in response")
	}
	fmt.Printf("result: %+v", result)
	id_user := strconv.Itoa(int(result["uid"].(float64)))
	fmt.Printf("uid: %v", id_user)

	// delete user
	req, _ = http.NewRequest(http.MethodDelete, "/v1/admin/userdel?uid="+id_user, nil)
	req.Header.Set("X-Service-Secret", serviceSec)

	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req)
	if resp2.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp2.Code)
	}
	var res map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &res)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	if _, ok := res["message"]; !ok {
		t.Error("Expected 'message' in response")
	}

}

func TestAdminUserpatch_DebugMode(t *testing.T) {
	router := setupTestServer()

	username, err := utils.GenerateRandomString(5)
	if err != nil {
		panic(err)
	}

	// register a dummy user
	body := fmt.Sprintf(`{"user":{"username":"%s", "info":"test@mail.com",
		"password":{"hashpass":"Testpass1"}}}`, username)
	req, _ := http.NewRequest(http.MethodPost, "/v1/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}

	var result map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	if _, ok := result["message"]; !ok {
		t.Error("Expected 'message' in response")
	}

	if _, ok := result["uid"]; !ok {
		t.Error("Expected 'uid' in response")
	}
	uid := strconv.Itoa(int(result["uid"].(float64)))

	// login
	body = fmt.Sprintf(`{"username":"%s","password":"Testpass1"}`, username)
	req, _ = http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req)

	if resp2.Code != http.StatusOK {
		t.Errorf("Expected 200 , got %d", resp2.Code)
	}
	var responseBody struct {
		User  utils.User `json:"user"`
		Token string     `json:"accessToken"`
	}

	err = json.Unmarshal(resp2.Body.Bytes(), &responseBody)
	if err != nil {
		t.Errorf("failed to unmarshal response body: %v", err)
	}

	if responseBody.Token == "" {
		t.Error("token was empty")
	}

	// patch user
	body = fmt.Sprintf(`{"uid":"%s", "info":"changed@mail.com","groups":"user,mod,admin"}`, uid)
	req, _ = http.NewRequest(http.MethodPatch, "/v1/admin/userpatch", bytes.NewBufferString(body))
	req.Header.Set("X-Service-Secret", serviceSec)
	resp25 := httptest.NewRecorder()
	router.ServeHTTP(resp25, req)
	if resp25.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp25.Code)
	}
	// check user
	req, _ = http.NewRequest(http.MethodGet, "/v1/user/me", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", responseBody.Token))
	resp3 := httptest.NewRecorder()
	router.ServeHTTP(resp3, req)
	if resp3.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp3.Code)
	}

	var res []map[string]any
	err = json.Unmarshal(resp3.Body.Bytes(), &res)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	fmt.Printf("response: %+v", res)
	if res[0]["info"].(string) != "changed@mail.com" {
		t.Errorf("Expected changed@mail.com info, got %v", res[0]["info"])
	}

}

func TestAdminUsermod_DebugMode(t *testing.T) {
	router := setupTestServer()

	username, err := utils.GenerateRandomString(5)
	if err != nil {
		panic(err)
	}

	// register a dummy user
	body := fmt.Sprintf(`{"user":{"username":"%s", "info":"test@mail.com",
		"password":{"hashpass":"Testpass1"}}}`, username)
	req, _ := http.NewRequest(http.MethodPost, "/v1/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}

	var result map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	if _, ok := result["message"]; !ok {
		t.Error("Expected 'message' in response")
	}

	if _, ok := result["uid"]; !ok {
		t.Error("Expected 'uid' in response")
	}

	fmt.Printf("user returned: %+v", result)

	// mod this user
	jsonData, _ := json.Marshal(map[string]any{"user": utils.User{
		Username: username,
		Info:     "modifiedmai@mod.user",
		Verified: false,
		Home:     "",
		Shell:    "",
		Password: utils.Password{Hashpass: "Testpass1"},
		Groups:   []utils.Group{{Groupname: username}, {Groupname: "user"}},
	}})

	req, _ = http.NewRequest(http.MethodPut, "/v1/admin/usermod", bytes.NewBuffer(jsonData))
	req.Header.Set("X-Service-Secret", serviceSec)
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req)
	if resp2.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp2.Code)
	}

	// login
	body = fmt.Sprintf(`{"username":"%s","password":"Testpass1"}`, username)
	req, _ = http.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp3 := httptest.NewRecorder()
	router.ServeHTTP(resp3, req)

	if resp3.Code != http.StatusOK {
		t.Errorf("Expected 200 , got %d", resp3.Code)
	}
	var responseBody struct {
		User  utils.User `json:"user"`
		Token string     `json:"accessToken"`
	}
	err = json.Unmarshal(resp3.Body.Bytes(), &responseBody)
	if err != nil {
		t.Errorf("failed to unmarshal response body: %v", err)
	}
	if responseBody.Token == "" {
		t.Error("token was empty")
	}

	// check user
	req, _ = http.NewRequest(http.MethodGet, "/v1/user/me", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", responseBody.Token))
	resp4 := httptest.NewRecorder()
	router.ServeHTTP(resp4, req)
	if resp4.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp4.Code)
	}

	var res []map[string]any
	err = json.Unmarshal(resp4.Body.Bytes(), &res)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}
	fmt.Printf("response: %+v", res)
	if res[0]["info"].(string) != "modifiedmai@mod.user" {
		t.Errorf("Expected modifiedmai@mod.user info, got %v", res[0]["info"])
	}
}

func TestAdminGroupadd_DebugMode(t *testing.T) {
	router := setupTestServer()
	groupname, err := utils.GenerateRandomString(5)
	if err != nil {
		panic(err)
	}
	// add a new group
	body := fmt.Sprintf(`{"groupname":"%s"}`, groupname)
	req, _ := http.NewRequest(http.MethodPost, "/v1/admin/groupadd", bytes.NewBufferString(body))
	req.Header.Set("X-Service-Secret", serviceSec)
	resp0 := httptest.NewRecorder()
	router.ServeHTTP(resp0, req)

	if resp0.Code != http.StatusOK && resp0.Code != http.StatusCreated {
		t.Errorf("Expected 200 or 201, got %d", resp0.Code)
	}

	// check for groups
	req, _ = http.NewRequest(http.MethodGet, "/v1/admin/groups", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", serviceSec)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}

	req, _ = http.NewRequest(http.MethodGet, "/v1/admin/groups", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", serviceSec)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}
	var result map[string][]map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}

	if len(result) == 0 {
		t.Errorf("Expected non empty result, got %v", result)
	}

	var flag = false
	for _, grp := range result["content"] {
		if grp["groupname"].(string) == groupname {
			flag = true

			break
		}
	}

	if !flag {
		t.Error("Expected to find the group in the result")
	}
}

func TestAdminGroupdel_DebugMode(t *testing.T) {
	router := setupTestServer()
	groupname, err := utils.GenerateRandomString(5)
	if err != nil {
		panic(err)
	}
	// add a new group
	body := fmt.Sprintf(`{"groupname":"%s"}`, groupname)
	req, _ := http.NewRequest(http.MethodPost, "/v1/admin/groupadd", bytes.NewBufferString(body))
	req.Header.Set("X-Service-Secret", serviceSec)
	resp0 := httptest.NewRecorder()
	router.ServeHTTP(resp0, req)

	if resp0.Code != http.StatusOK && resp0.Code != http.StatusCreated {
		t.Errorf("Expected 200 or 201, got %d", resp0.Code)
	}
	var res map[string]any
	err = json.Unmarshal(resp0.Body.Bytes(), &res)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}

	if _, ok := res["gid"]; !ok {
		t.Error("Expected 'gid' in response")
	}

	// delete the group
	req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/admin/groupdel?gid=%v", res["gid"]), nil)
	req.Header.Set("X-Service-Secret", serviceSec)
	resp01 := httptest.NewRecorder()
	router.ServeHTTP(resp01, req)
	if resp01.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp01.Code)
	}

	// check for groups
	req, _ = http.NewRequest(http.MethodGet, "/v1/admin/groups", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", serviceSec)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}

	req, _ = http.NewRequest(http.MethodGet, "/v1/admin/groups", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", serviceSec)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.Code)
	}
	var result map[string][]map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("JSON unmarshal failed: %v", err)
	}

	if len(result) == 0 {
		t.Errorf("Expected non empty result, got %v", result)
	}

	for _, grp := range result["content"] {
		if grp["groupname"].(string) == groupname {
			t.Errorf("Expected not to find the groupname after deletion: %v", groupname)
		}
	}

}
