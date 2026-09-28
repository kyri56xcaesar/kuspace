package fslite

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testServiceSecret = "service-secret"

// newTestServer is a standalone fslite server with one volume ("volume1",
// files on a temp dir) and the admin account admin / adminpassword1.
func newTestServer(t *testing.T) (*FsLite, *gin.Engine) {
	t.Helper()
	fsl := NewFsLite(ut.EnvConfig{
		ServiceConfig: ut.ServiceConfig{APIGinMode: "test"},
		AuthConfig:    ut.AuthConfig{JwtSecretKey: []byte("jwt-key"), ServiceSecretKey: []byte(testServiceSecret)},
		FsliteConfig: ut.FsliteConfig{
			FslDB: "fsl_test.db", FslDBPath: t.TempDir() + "/", FslDBDriver: "sqlite3",
			FslDBMaxOpenConns: "4", FslDBMaxIdleConns: "2", FslDBMaxLifetime: "5",
			FslAccessKey: "admin", FslSecretKey: "adminpassword1",
			FslServer: true,
		},
	})
	t.Cleanup(fsl.Close)
	mustVolume(t, &fsl, "volume1", 0)
	withLocality(t, &fsl, "volume1")

	return &fsl, fsl.routes()
}

func do(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func login(t *testing.T, h http.Handler, user, password string) (int, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": user, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := do(t, h, req)
	var out struct{ Token string }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)

	return rec.Code, out.Token
}

func uploadReq(t *testing.T, volume, name, content, token string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("files", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte(content))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/admin/resource/upload?volume="+volume, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	return req
}

func TestServerAuthentication(t *testing.T) {
	_, h := newTestServer(t)

	if rec := do(t, h, httptest.NewRequest(http.MethodGet, "/health", nil)); rec.Code != http.StatusOK {
		t.Errorf("health = %d", rec.Code)
	}

	cases := map[string]func(*http.Request){
		"no credentials":       func(*http.Request) {},
		"wrong service secret": func(r *http.Request) { r.Header.Set("X-Service-Secret", "nope") }, // was an empty 200
		"short header":         func(r *http.Request) { r.Header.Set("Authorization", "Bad") },     // was a panic
		"garbage token":        func(r *http.Request) { r.Header.Set("Authorization", "Bearer x.y.z") },
		"a JWT signed with the raw key": func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+mustSign(t, []byte("jwt-key")))
		},
	}
	for name, set := range cases {
		req := httptest.NewRequest(http.MethodGet, "/admin/volume/get", nil)
		set(req)
		if rec := do(t, h, req); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: %d, want 401", name, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/volume/get", nil)
	req.Header.Set("X-Service-Secret", testServiceSecret)
	if rec := do(t, h, req); rec.Code != http.StatusOK {
		t.Errorf("service secret: %d, want 200", rec.Code)
	}

	if code, _ := login(t, h, "admin", "wrong-password"); code != http.StatusForbidden {
		t.Errorf("wrong password: %d, want 403", code)
	}
	code, token := login(t, h, "admin", "adminpassword1")
	if code != http.StatusOK || token == "" {
		t.Fatalf("login: %d %q", code, token)
	}
	req = httptest.NewRequest(http.MethodGet, "/admin/volume/get", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if rec := do(t, h, req); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "volume1") {
		t.Errorf("token: %d %s", rec.Code, rec.Body)
	}
}

func TestServerUploadDownload(t *testing.T) {
	_, h := newTestServer(t)
	_, token := login(t, h, "admin", "adminpassword1")

	// uploads carried volume id 0 and were all rejected by the foreign key
	if rec := do(t, h, uploadReq(t, "volume1", "notes.txt", "hello fslite", token)); rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	// duplicates were answered 200 (the insert error was overwritten)
	if rec := do(t, h, uploadReq(t, "volume1", "notes.txt", "again", token)); rec.Code != http.StatusConflict {
		t.Errorf("duplicate upload: %d, want 409", rec.Code)
	}
	if rec := do(t, h, uploadReq(t, "nowhere", "x.txt", "x", token)); rec.Code != http.StatusNotFound {
		t.Errorf("upload to a missing volume: %d, want 404", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/resource/download?resource=volume1/notes.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if rec := do(t, h, req); rec.Code != http.StatusOK || rec.Body.String() != "hello fslite" {
		t.Errorf("download: %d %q", rec.Code, rec.Body)
	}
}

// mustSign signs a well-formed fslite admin token with key.
func mustSign(t *testing.T, key []byte) string {
	t.Helper()
	claims := CustomClaims{ID: "0", Username: "admin", RegisteredClaims: jwt.RegisteredClaims{
		Issuer: "fslite", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	return s
}
