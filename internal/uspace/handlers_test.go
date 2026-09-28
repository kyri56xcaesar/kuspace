package uspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"kyri56xcaesar/kuspace/internal/authn"
	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// memStorage is an in-memory StorageSystem, so handlers run without MinIO.
type memStorage struct {
	mu      sync.Mutex
	objects map[string][]byte // "volume/name"
}

func key(vname, name string) string { return vname + "/" + strings.TrimPrefix(name, "/") }

func (m *memStorage) DefaultVolume(bool) string               { return "vol1" }
func (m *memStorage) CreateVolume(context.Context, any) error { return nil }
func (m *memStorage) SelectVolumes(context.Context, map[string]any) (any, error) {
	return nil, nil
}
func (m *memStorage) SelectObjects(context.Context, map[string]any) (any, error) {
	return nil, nil
}
func (m *memStorage) RemoveVolume(context.Context, any) error         { return nil }
func (m *memStorage) Update(context.Context, map[string]string) error { return nil }
func (m *memStorage) Share(context.Context, string, any) (any, error) { return nil, nil }
func (m *memStorage) Stat(context.Context, any) (any, error) {
	return nil, errors.New("not in tests")
}

func (m *memStorage) Insert(_ context.Context, t any) error {
	r := t.(ut.Resource)
	b, err := io.ReadAll(r.Reader)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key(r.Vname, r.Name)] = b

	return nil
}

func (m *memStorage) Download(_ context.Context, t *any) (context.CancelFunc, error) {
	r := (*t).(*ut.Resource)
	m.mu.Lock()
	b, ok := m.objects[key(r.Vname, r.Name)]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("%w: no such object", ut.ErrNotFound) // as the MinIO client maps NoSuchKey
	}
	r.Reader, r.Size = bytes.NewReader(b), int64(len(b))

	return func() {}, nil
}

func (m *memStorage) Remove(_ context.Context, t any) error {
	r := t.(ut.Resource)
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key(r.Vname, r.Name))

	return nil
}

func (m *memStorage) Copy(_ context.Context, s, d any) error {
	src, dst := s.(ut.Resource), d.(ut.Resource)
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key(src.Vname, src.Name)]
	if !ok {
		return errors.New("no such object")
	}
	m.objects[key(dst.Vname, dst.Name)] = b

	return nil
}

const svcSecret = "svc-secret"

// apiHarness is uspace's real router over an in-memory store; callers act
// like frontapp (service secret + Access-Target).
type apiHarness struct {
	t   *testing.T
	srv *UService
	h   http.Handler
}

func newAPIHarness(t *testing.T, quotaGB float64) *apiHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	srv := newTestService(t)
	srv.config.ServiceSecretKey = []byte(svcSecret)
	srv.config.LocalVolumesDefaultCapacity = quotaGB
	srv.config.MinioDefaultBucket = "vol1"
	srv.storage = &memStorage{objects: map[string][]byte{}}
	srv.Engine = gin.New()
	srv.RegisterRoutes()
	for _, v := range []string{"team", "other"} {
		if err := srv.fsl.CreateVolume(t.Context(), ut.Volume{Name: v, CreatedAt: ut.CurrentTime()}); err != nil {
			t.Fatal(err)
		}
	}

	return &apiHarness{t: t, srv: srv, h: srv.Engine}
}

// do sends a request as who ("uid:gids"; "" = no identity) on target
// ("vname:path").
func (a *apiHarness) do(method, path, target, who string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequestWithContext(a.t.Context(), method, path, body)
	req.Header.Set("X-Service-Secret", svcSecret)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if who != "" {
		req.Header.Set("Access-Target", "0:"+target+" "+who)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)

	return rec
}

func (a *apiHarness) upload(volume, name, content, who string) *httptest.ResponseRecorder {
	a.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("files", name)
	_, _ = fw.Write([]byte(content))
	_ = mw.Close()

	return a.do(http.MethodPost, "/api/v1/resource/upload", volume+":/", who, &buf, mw.FormDataContentType())
}

func (a *apiHarness) resource(volume, name string) ut.Resource {
	a.t.Helper()
	r, found, err := a.srv.lookupResource(a.t.Context(), name, volume)
	if err != nil || !found {
		a.t.Fatalf("%s/%s not recorded: %v", volume, name, err)
	}

	return r
}

func (a *apiHarness) assignGroup(volume string, gid int64, quotaGB float64) *httptest.ResponseRecorder {
	a.t.Helper()
	body := fmt.Sprintf(`{"vname":%q,"gid":%d,"quota":%v}`, volume, gid, quotaGB)

	return a.do(http.MethodPost, "/api/v1/admin/group/volume", "::/", "0:0", strings.NewReader(body), "application/json")
}

func TestUploadHandler(t *testing.T) {
	a := newAPIHarness(t, 0.000001) // 1 kB personal quota

	if rec := a.upload("vol1", "a.txt", "hello", "1001:1002,1003"); rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	if r := a.resource("vol1", "a.txt"); r.UID != 1001 || r.GID != 1002 || r.Size != 5 {
		t.Errorf("recorded %+v: want owner 1001, primary group 1002, size 5", r)
	}
	if rec := a.upload("vol1", "a.txt", "again", "1001:1002"); rec.Code != http.StatusConflict {
		t.Errorf("duplicate: %d, want 409", rec.Code)
	}
	if rec := a.upload("vol1", "big.bin", strings.Repeat("x", 2000), "1001:1002"); rec.Code != http.StatusInsufficientStorage {
		t.Errorf("past the quota: %d, want 507", rec.Code)
	}
	if rec := a.do(http.MethodPost, "/api/v1/resource/upload", "", "", nil, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("no Access-Target: %d, want 400", rec.Code)
	}

	// read access follows the permissions (rw-r-----)
	if rec := a.do(http.MethodGet, "/api/v1/resource/download", "vol1:/a.txt", "1001:1002", nil, ""); rec.Code != http.StatusOK || rec.Body.String() != "hello" {
		t.Errorf("owner download: %d %q", rec.Code, rec.Body)
	}
	if rec := a.do(http.MethodGet, "/api/v1/resource/download", "vol1:/a.txt", "2002:2002", nil, ""); rec.Code != http.StatusForbidden {
		t.Errorf("stranger download: %d, want 403", rec.Code)
	}
}

func TestGroupVolumeHandlers(t *testing.T) {
	a := newAPIHarness(t, 0.000001) // members' personal quota: 1 kB

	const member, member2, stranger = "1001:1001,500", "3003:3003,500", "2002:2002"
	if rec := a.assignGroup("vol1", 500, 1); rec.Code != http.StatusForbidden {
		t.Errorf("default volume as a group volume: %d, want 403", rec.Code)
	}
	if rec := a.assignGroup("team", 500, 0.00001); rec.Code != http.StatusOK { // 10 kB for the group
		t.Fatalf("assign: %d %s", rec.Code, rec.Body)
	}

	// a member writes more than their own 1 kB quota: the group pays
	if rec := a.upload("team", "shared.csv", strings.Repeat("x", 4000), member); rec.Code != http.StatusOK {
		t.Fatalf("member upload: %d %s", rec.Code, rec.Body)
	}
	if r := a.resource("team", "shared.csv"); r.GID != 500 {
		t.Errorf("file group = %d, want the volume's group 500", r.GID)
	}
	if rec := a.upload("team", "x.csv", "x", stranger); rec.Code != http.StatusForbidden {
		t.Errorf("stranger upload: %d, want 403", rec.Code)
	}
	if rec := a.upload("team", "more.csv", strings.Repeat("y", 7000), member2); rec.Code != http.StatusInsufficientStorage {
		t.Errorf("past the group quota: %d, want 507", rec.Code)
	}

	// members read each other's files; strangers don't
	if rec := a.do(http.MethodGet, "/api/v1/resource/download", "team:/shared.csv", member2, nil, ""); rec.Code != http.StatusOK {
		t.Errorf("other member download: %d, want 200", rec.Code)
	}
	if rec := a.do(http.MethodGet, "/api/v1/resource/download", "team:/shared.csv", stranger, nil, ""); rec.Code != http.StatusForbidden {
		t.Errorf("stranger download: %d, want 403", rec.Code)
	}

	// copying into the group volume is for members only
	if rec := a.upload("vol1", "mine.txt", "m", "2002:2002"); rec.Code != http.StatusOK {
		t.Fatalf("stranger's own upload: %d", rec.Code)
	}
	if rec := a.do(http.MethodPost, "/api/v1/resource/cp?dest=team/mine.txt", "vol1:/mine.txt", stranger, nil, ""); rec.Code != http.StatusForbidden {
		t.Errorf("stranger copy into the group volume: %d, want 403", rec.Code)
	}

	// who sees which shared volumes
	var gvs []ut.GroupVolume
	rec := a.do(http.MethodGet, "/api/v1/volumes/shared", "::/", member, nil, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &gvs)
	if len(gvs) != 1 || gvs[0].Vname != "team" || gvs[0].Usage <= 0 {
		t.Errorf("member's shared volumes = %s", rec.Body)
	}
	rec = a.do(http.MethodGet, "/api/v1/volumes/shared", "::/", stranger, nil, "")
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("stranger's shared volumes = %s", rec.Body)
	}

	// admin API guards
	if rec := a.do(http.MethodPost, "/api/v1/admin/group/volume", "::/", "1001:1001", strings.NewReader(`{}`), "application/json"); rec.Code == http.StatusOK {
		t.Error("admin API accepted a request without vname/gid")
	}
	if rec := a.do(http.MethodDelete, "/api/v1/admin/group/volume?volume=team", "::/", "0:0", nil, ""); rec.Code != http.StatusConflict {
		t.Errorf("release while holding files: %d, want 409", rec.Code)
	}
	if rec := a.do(http.MethodDelete, "/api/v1/admin/group/volume?volume=other", "::/", "0:0", nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("release of a non-group volume: %d, want 404", rec.Code)
	}
}

// withJobsDB gives the harness a jobs database holding one job per uid.
func (a *apiHarness) withJobsDB(uids ...int64) {
	a.t.Helper()
	a.srv.jdbh = ut.NewDBHandler("jobs_test.db", a.t.TempDir()+"/", "sqlite3")
	a.srv.jdbh.Init(initSQLJobs, "4", "2", "5")
	a.t.Cleanup(a.srv.jdbh.Close)
	if err := a.srv.ensureJobColumns(a.t.Context()); err != nil {
		a.t.Fatal(err)
	}
	for _, uid := range uids {
		if _, err := a.srv.insertJob(a.t.Context(), ut.Job{UID: uid, Input: "vol1/in.csv", Output: "vol1/out.csv",
			Logic: "bash", LogicBody: "secret code of " + strconv.FormatInt(uid, 10), Status: "completed", CreatedAt: ut.CurrentTime()}); err != nil {
			a.t.Fatal(err)
		}
	}
}

// Users list their own jobs only; any user used to be able to list
// everyone's (code and file paths included).
func TestJobListingIsScopedToTheCaller(t *testing.T) {
	a := newAPIHarness(t, 1)
	a.withJobsDB(1001, 2002, 2002)
	jobsOf := func(who, query string) []ut.Job {
		t.Helper()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/job"+query, nil)
		req.Header.Set("X-Service-Secret", svcSecret)
		req.Header.Set("Access-Target", "0::/ "+who)
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, req)
		var body struct {
			Content []ut.Job `json:"content"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)

		return body.Content
	}
	// frontapp as a service is privileged; users come with their own token
	a.srv.config.JwtSecretKey = []byte("jwt")
	a.srv.tokens = nil
	tok := func(uid string) string {
		c := &authn.Claims{UserID: uid, Groups: "user", GroupIDs: uid,
			RegisteredClaims: jwt.RegisteredClaims{Issuer: "minioth", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
		s, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte("jwt"))

		return s
	}
	asUser := func(uid, query string) (int, []ut.Job) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/job"+query, nil)
		req.Header.Set("Authorization", "Bearer "+tok(uid))
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, req)
		var body struct {
			Content []ut.Job `json:"content"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)

		return rec.Code, body.Content
	}

	for _, query := range []string{"", "?uid=2002", "?uids=2002", "?jids=*"} {
		code, jobs := asUser("1001", query)
		if code != http.StatusOK || len(jobs) != 1 || jobs[0].UID != 1001 {
			t.Errorf("user 1001 with %q: %d, %d job(s) %+v", query, code, len(jobs), jobs)
		}
	}
	if jobs := jobsOf("0:0", ""); len(jobs) != 3 {
		t.Errorf("service listing: %d jobs, want 3", len(jobs))
	}
}

// Volumes made through the admin API get a record as well as a bucket:
// uploads to them used to fail the resources' foreign key.
func TestAdminVolumesAreUsable(t *testing.T) {
	a := newAPIHarness(t, 1)
	create := func(body string) int {
		return a.do(http.MethodPost, "/api/v1/admin/volumes", "::/", "0:0", strings.NewReader(body), "application/json").Code
	}
	if code := create(`{"name":"lab-data","capacity":1}`); code != http.StatusCreated {
		t.Fatalf("create: %d", code)
	}
	if code := create(`{"name":"lab-data","capacity":1}`); code != http.StatusConflict {
		t.Errorf("duplicate: %d, want 409", code)
	}
	if rec := a.upload("lab-data", "x.csv", "1,2", "1001:1001"); rec.Code != http.StatusOK {
		t.Errorf("upload to the new volume: %d %s", rec.Code, rec.Body)
	}
	rec := a.do(http.MethodGet, "/api/v1/admin/volumes", "::/", "0:0", nil, "")
	if !strings.Contains(rec.Body.String(), `"lab-data"`) || !strings.Contains(rec.Body.String(), `"vid"`) {
		t.Errorf("list: %s", rec.Body)
	}
	if _, err := a.srv.fsl.AssignGroupVolume(t.Context(), "other", 500, 1); err != nil {
		t.Fatal(err)
	}
	if rec := a.do(http.MethodGet, "/api/v1/admin/volumes", "::/", "0:0", nil, ""); !strings.Contains(rec.Body.String(), `"shared":true,"gid":500`) {
		t.Errorf("group volume not marked: %s", rec.Body)
	}
	if rec := a.do(http.MethodDelete, "/api/v1/admin/volumes?volume=vol1", "::/", "0:0", nil, ""); rec.Code != http.StatusForbidden {
		t.Errorf("deleting the default volume: %d, want 403", rec.Code)
	}
	if rec := a.do(http.MethodDelete, "/api/v1/admin/volumes?volume=team", "::/", "0:0", nil, ""); rec.Code != http.StatusAccepted {
		t.Errorf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do(http.MethodGet, "/api/v1/admin/volumes", "::/", "0:0", nil, ""); strings.Contains(rec.Body.String(), `"team"`) {
		t.Errorf("deleted volume still listed: %s", rec.Body)
	}
}

// Errors answer with the status their kind maps to, whatever their text.
func TestErrorsAnswerByKind(t *testing.T) {
	a := newAPIHarness(t, 1)
	// a record whose object is gone (the kind of drift fsck reports)
	now := ut.CurrentTime()
	if err := a.srv.fsl.Insert(t.Context(), ut.Resource{Name: "/ghost.txt", Vname: "vol1", VID: a.srv.volumeID(t.Context(), "vol1"),
		UID: 1001, GID: 1001, Perms: ut.DefaultFilePerms, Type: "file", CreatedAt: now, UpdatedAt: now, AccessedAt: now}); err != nil {
		t.Fatal(err)
	}
	if rec := a.do(http.MethodGet, "/api/v1/resource/download", "vol1:/ghost.txt", "1001:1001", nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("missing object: %d, want 404 (was 400)", rec.Code)
	}
	// no files is an empty list (was 404)
	rec := a.do(http.MethodGet, "/api/v1/resources", "team:/", "2002:2002", nil, "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("empty listing: %d %s", rec.Code, rec.Body)
	}
	if rec := a.upload("vol1", "ghost.txt", "x", "1001:1001"); rec.Code != http.StatusConflict {
		t.Errorf("taken name: %d, want 409", rec.Code)
	}
}

type brokenStorage struct{ *memStorage }

func (brokenStorage) SelectVolumes(context.Context, map[string]any) (any, error) {
	return nil, fmt.Errorf("minio: %w", ut.ErrUnavailable)
}

func TestReadiness(t *testing.T) {
	a := newAPIHarness(t, 1)
	a.withJobsDB()
	rec := a.do(http.MethodGet, "/readyz", "", "", nil, "")
	if rec.Code != http.StatusOK {
		t.Errorf("ready: %d %s", rec.Code, rec.Body)
	}
	a.srv.storage = brokenStorage{&memStorage{objects: map[string][]byte{}}}
	rec = a.do(http.MethodGet, "/readyz", "", "", nil, "")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"metadata":"ok"`) {
		t.Errorf("storage down: %d %s", rec.Code, rec.Body)
	}
}

// Without a cluster the metrics say so (building clients from a nil
// config used to panic: a 500) and still report uspace's own process.
func TestSystemMetricsWithoutKubernetes(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no kubeconfig
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	a := newAPIHarness(t, 1)
	rec := a.do(http.MethodGet, "/api/v1/admin/system-metrics", "::/", "0:0", nil, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"available":false`) || !strings.Contains(rec.Body.String(), `"goroutines"`) {
		t.Errorf("metrics: %d %s", rec.Code, rec.Body)
	}
}

func TestUserQuotaAdmin(t *testing.T) {
	a := newAPIHarness(t, 0.000001) // 1 kB default quota
	admin := func(method, path, body string) *httptest.ResponseRecorder {
		return a.do(method, path, "::/", "0:0", strings.NewReader(body), "application/json")
	}
	if rec := a.upload("vol1", "big.bin", strings.Repeat("x", 2000), "1001:1001"); rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("over the default quota: %d", rec.Code)
	}
	if rec := admin(http.MethodPatch, "/api/v1/admin/user/volume", `{"vname":"vol1","uid":1001,"quota":0.00001}`); rec.Code != http.StatusOK {
		t.Fatalf("raise quota: %d %s", rec.Code, rec.Body)
	}
	if rec := a.upload("vol1", "big.bin", strings.Repeat("x", 2000), "1001:1001"); rec.Code != http.StatusOK {
		t.Errorf("after raising the quota: %d %s", rec.Code, rec.Body)
	}
	rec := admin(http.MethodGet, "/api/v1/admin/user/volume?uids=1001", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"vname":"vol1"`) || !strings.Contains(rec.Body.String(), `"quota":0.00001`) {
		t.Errorf("list: %d %s", rec.Code, rec.Body)
	}
	if rec := admin(http.MethodPatch, "/api/v1/admin/user/volume", `{"vname":"vol1","uid":1001,"quota":-1}`); rec.Code != http.StatusBadRequest {
		t.Errorf("negative quota: %d", rec.Code)
	}
	if rec := admin(http.MethodDelete, "/api/v1/admin/user/volume?volume=vol1&uid=1001", ""); rec.Code != http.StatusOK {
		t.Errorf("reset: %d %s", rec.Code, rec.Body)
	}
	if rec := admin(http.MethodDelete, "/api/v1/admin/user/volume?volume=vol1&uid=1001", ""); rec.Code != http.StatusNotFound {
		t.Errorf("second reset: %d", rec.Code)
	}
	// a plain user can't manage quotas
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/admin/user/volume", nil)
	req.Header.Set("Authorization", "Bearer not-a-token")
	rec = httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated: %d", rec.Code)
	}
}
