package frontendapp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

/*
	Jobs and applications. Calls go to uspace with the user's token: uspace
	shows users their own jobs (admins everyone's), checks job input/output
	permissions and owns cancellation.
*/

// newestFirst compares two ut.TimeFormat timestamps.
func newestFirst(a, b string) bool {
	t1, err1 := time.Parse(ut.TimeFormat, a)
	t2, err2 := time.Parse(ut.TimeFormat, b)

	return err1 == nil && err2 == nil && t1.After(t2)
}

func sortJobs(jobs []ut.Job, by string) {
	less := map[string]func(a, b ut.Job) bool{
		"output": func(a, b ut.Job) bool { return a.Output > b.Output },
		"uid":    func(a, b ut.Job) bool { return a.UID > b.UID },
		"jid":    func(a, b ut.Job) bool { return a.JID > b.JID },
		"status": func(a, b ut.Job) bool { return compareStatus(a.Status, b.Status) },
	}[by]
	if less == nil { // "createdAt", "time" and the default
		less = func(a, b ut.Job) bool { return newestFirst(a.CreatedAt, b.CreatedAt) }
	}
	sort.SliceStable(jobs, func(i, j int) bool { return less(jobs[i], jobs[j]) })
}

func sortApps(apps []ut.Application, by string) {
	less := map[string]func(a, b ut.Application) bool{
		"name":       func(a, b ut.Application) bool { return a.Name > b.Name },
		"image":      func(a, b ut.Application) bool { return a.Image > b.Image },
		"version":    func(a, b ut.Application) bool { return a.Version > b.Version },
		"status":     func(a, b ut.Application) bool { return compareStatus(a.Status, b.Status) },
		"author":     func(a, b ut.Application) bool { return a.Author > b.Author },
		"author_id":  func(a, b ut.Application) bool { return a.AuthorID > b.AuthorID },
		"insertedAt": func(a, b ut.Application) bool { return newestFirst(a.InsertedAt, b.InsertedAt) },
	}[by]
	if less == nil {
		less = func(a, b ut.Application) bool { return newestFirst(a.CreatedAt, b.CreatedAt) }
	}
	sort.SliceStable(apps, func(i, j int) bool { return less(apps[i], apps[j]) })
}

// listJobs renders the jobs the user may see (theirs; everyone's for admins).
func (srv *HTTPService) listJobs(c *gin.Context, withAdminFlag bool) {
	var resp struct {
		Content []ut.Job `json:"content"`
		Admin   int      `json:"admin"`
	}
	err := srv.uspace.decode(c.Request.Context(), request{
		method: http.MethodGet, path: "/api/v1/job", token: userToken(c), target: "0::/",
	}, &resp)
	if err != nil {
		fail(c, "list jobs", err)

		return
	}
	sortJobs(resp.Content, c.Query("sort"))
	if !withAdminFlag {
		respondInFormat(c, c.Query("format"), resp.Content, "jobs_list_template.html")

		return
	}
	if isAdmin(c) {
		resp.Admin = 1
	}
	respondInFormat(c, c.Query("format"), resp, "jobs_list_template.html")
}

// bindJob reads and checks a submitted job; the owner is the session user.
func (srv *HTTPService) bindJob(c *gin.Context) (ut.Job, bool) {
	var job ut.Job
	if err := c.ShouldBind(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind"})

		return job, false
	}
	cfg := srv.Config
	if err := job.ValidateForm(cfg.UspaceJobMaxCPU, cfg.UspaceJobMaxMemory, cfg.UspaceJobMaxStorage,
		int64(cfg.UspaceJobMaxParallelism), cfg.UspaceJobMaxTimeout, cfg.UspaceJobMaxLogicSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return job, false
	}
	uid, err := strconv.ParseInt(c.GetString("userID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bad session"})

		return job, false
	}
	job.UID = uid

	return job, true
}

// submitJob sends a job to uspace (path: the user or the admin endpoint).
func (srv *HTTPService) submitJob(c *gin.Context, path string) {
	job, ok := srv.bindJob(c)
	if !ok {
		return
	}
	var resp struct {
		JID    int    `json:"jid"`
		Status string `json:"status"`
	}
	err := srv.uspace.decode(c.Request.Context(), request{
		method: http.MethodPost, path: path, json: job, token: userToken(c), target: "0::/",
	}, &resp)
	if err != nil {
		fail(c, "submit job", err)

		return
	}
	c.JSON(http.StatusOK, gin.H{"jid": resp.JID, "status": resp.Status, "output": job.Output})
}

func (srv *HTTPService) jobsHandler(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		srv.listJobs(c, true)
	case http.MethodPost:
		srv.submitJob(c, "/api/v1/job")
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not supported"})
	}
}

func (srv *HTTPService) jobAdminHandler(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		srv.listJobs(c, false)
	case http.MethodPost: // as root: no input/output permission checks
		srv.submitJob(c, "/api/v1/admin/job")
	case http.MethodPut:
		job, ok := srv.bindJob(c)
		if !ok {
			return
		}
		srv.relayCall(c, "update job", request{method: http.MethodPut, path: "/api/v1/admin/job", json: job})
	case http.MethodDelete:
		jid := c.Query("jid")
		if jid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a jid"})

			return
		}
		srv.relayCall(c, "delete job", request{method: http.MethodDelete, path: "/api/v1/admin/job", query: url.Values{"jid": {jid}}})
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not supported"})
	}
}

func (srv *HTTPService) listApps(c *gin.Context, withAdminFlag bool) {
	var resp struct {
		Content []ut.Application `json:"content"`
		Admin   int              `json:"admin"`
	}
	err := srv.uspace.decode(c.Request.Context(), request{
		method: http.MethodGet, path: "/api/v1/app", token: userToken(c), target: "0::/",
	}, &resp)
	if err != nil {
		fail(c, "list apps", err)

		return
	}
	sortApps(resp.Content, c.Query("sort"))
	if !withAdminFlag {
		respondInFormat(c, c.Query("format"), resp.Content, "apps_list_template.html")

		return
	}
	if isAdmin(c) {
		resp.Admin = 1
	}
	respondInFormat(c, c.Query("format"), resp, "apps_list_template.html")
}

func (srv *HTTPService) appsHandler(c *gin.Context) {
	if c.Request.Method != http.MethodGet {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not supported"})

		return
	}
	srv.listApps(c, true)
}

func (srv *HTTPService) appAdminHandler(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		srv.listApps(c, false)
	case http.MethodPost, http.MethodPut:
		var app ut.Application
		if err := c.ShouldBind(&app); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind"})

			return
		}
		srv.relayCall(c, "save app", request{method: c.Request.Method, path: "/api/v1/admin/app", json: app})
	case http.MethodDelete:
		id := c.Query("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a id"})

			return
		}
		srv.relayCall(c, "delete app", request{method: http.MethodDelete, path: "/api/v1/admin/app", query: url.Values{"id": {id}}})
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not supported"})
	}
}

// relayCall makes r to uspace as the session user and relays the answer.
func (srv *HTTPService) relayCall(c *gin.Context, what string, r request) {
	r.token, r.target = userToken(c), "0::/"
	resp, err := srv.uspace.do(c.Request.Context(), r)
	if err != nil {
		fail(c, what, err)

		return
	}
	relay(c, resp)
}

// jobOwner asks uspace, as a service, who owns job jid.
func (srv *HTTPService) jobOwner(ctx context.Context, jid int) (int64, error) {
	var resp struct {
		Content ut.Job `json:"content"`
	}
	err := srv.uspace.decode(ctx, request{
		method: http.MethodGet, path: "/api/v1/job", query: url.Values{"jids": {strconv.Itoa(jid)}},
		svc: true, target: "0::/ 0:0",
	}, &resp)

	return resp.Content.UID, err
}

// handleWSTicket issues a one-minute ticket to watch a job's live output on
// wss (which can't see this session itself): the job's owner or an admin.
func (srv *HTTPService) handleWSTicket(c *gin.Context) {
	jid, err := strconv.Atoi(c.Query("jid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jid must be a number"})

		return
	}
	if !isAdmin(c) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		owner, err := srv.jobOwner(ctx, jid)
		if err != nil {
			fail(c, "find the job", err)

			return
		}
		if strconv.FormatInt(owner, 10) != c.GetString("userID") {
			c.JSON(http.StatusForbidden, gin.H{"error": "you can only watch your own jobs"})

			return
		}
	}
	ticket := ut.SignWSTicket(srv.Config.ServiceSecretKey, strconv.Itoa(jid), "consumer", c.GetString("userID"), time.Now())
	c.JSON(http.StatusOK, gin.H{"ticket": ticket, "expiresIn": int(ut.WSTicketTTL.Seconds())})
}

// jobCall relays a per-job uspace call (log, cancel) as the session user;
// uspace allows the owner and admins.
func (srv *HTTPService) jobCall(c *gin.Context, method, path string) {
	jid, err := strconv.Atoi(c.Query("jid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jid must be a number"})

		return
	}
	srv.relayCall(c, fmt.Sprintf("job %d", jid), request{method: method, path: path, query: url.Values{"jid": {strconv.Itoa(jid)}}})
}

// handleJobLog returns the saved output of one of the user's jobs.
func (srv *HTTPService) handleJobLog(c *gin.Context) {
	srv.jobCall(c, http.MethodGet, "/api/v1/job/log")
}

// handleJobCancel cancels one of the user's queued or running jobs.
func (srv *HTTPService) handleJobCancel(c *gin.Context) {
	srv.jobCall(c, http.MethodPost, "/api/v1/job/cancel")
}
