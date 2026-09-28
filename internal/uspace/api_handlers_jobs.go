package uspace

/*
	http api handlers for the uspace service
	"job" related endpoints

	@connected to:
	-> database calls - DatabaseHandler
	-> publishing/subscribing "jobs" to execution - Broker
*/

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/api/resource"
)

// errJobForbidden marks an authorization failure (403) as opposed to a lookup
// error (500).
var errJobForbidden = errors.New("forbidden")

// authorizeJobIO checks that the submitter may read the job's input and, if
// the output already exists, overwrite it. The executor reads and writes
// storage with service credentials, so without this check a job could read
// another user's private file or overwrite it. The submitter's identity comes
// from the Access-Target header set by frontapp from the verified token.
func (srv *UService) authorizeJobIO(c *gin.Context, job ut.Job) error {
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil {
		return fmt.Errorf("%w: missing submitter identity", errJobForbidden)
	}
	if ac.UID == "0" {
		return nil
	}
	if ac.UID != strconv.FormatInt(job.UID, 10) {
		return fmt.Errorf("%w: job owner does not match submitter", errJobForbidden)
	}

	split := func(p string) (string, string, bool) {
		volume, name, ok := strings.Cut(strings.TrimPrefix(p, "/"), "/")

		return volume, name, ok && volume != "" && name != ""
	}

	vol, name, ok := split(job.Input)
	if !ok {
		return fmt.Errorf("%w: input must be <volume>/<object>", errJobForbidden)
	}
	in, found, err := srv.lookupResource(c.Request.Context(), name, vol)
	if err != nil {
		return err
	}
	if !found || !in.HasAccess(ac) {
		// same answer for "missing" and "not yours", so jobs can't probe for files
		return fmt.Errorf("%w: no read access to input %s", errJobForbidden, job.Input)
	}

	vol, name, ok = split(job.Output)
	if !ok {
		return fmt.Errorf("%w: output must be <volume>/<object>", errJobForbidden)
	}
	out, found, err := srv.lookupResource(c.Request.Context(), name, vol)
	if err != nil {
		return err
	}
	if found && !out.HasWriteAccess(ac) {
		return fmt.Errorf("%w: no write access to output %s", errJobForbidden, job.Output)
	}
	if _, err := srv.writeGroup(c.Request.Context(), ac, vol, job.UID); errors.Is(err, errNotGroupMember) {
		return fmt.Errorf("%w: output %s is on a group volume you're not a member of", errJobForbidden, job.Output)
	} else if err != nil {
		return err
	}

	return nil
}

func respondJobAuthError(c *gin.Context, err error) {
	if errors.Is(err, errJobForbidden) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})

		return
	}
	log.Printf("failed to authorize job io: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check job input/output"})
}

// Api call Handlers
// HandleJob handles job creation (POST) and job querying (GET)
//
// @Summary     Get or submit jobs
// @Description GET retrieves jobs by uid(s), jid, or returns all. POST submits one or multiple jobs.
// @Tags        jobs
// @Accept      json
// @Produce     json
//
// @Param       limit   query     string  false  "Limit number of jobs"
// @Param       offset  query     string  false  "Offset for pagination"
// @Param       uids    query     string  false  "Comma-separated list of user IDs"
// @Param       jids    query     string  false  "Job ID or '*' for all jobs"
//
// @Param       job     body      ut.Job     true  "Single job"      default({"uid":1,"input":"...","meta":"..."})
// @Param       jobs    body      []ut.Job   true  "Multiple jobs"   default([{"uid":1},{"uid":2}])
//
// @Success     200     {object}  map[string]interface{}
// @Failure     400     {object}  map[string]string
// @Failure     403     {object}  map[string]string
// @Failure     405     {object}  map[string]string
// @Failure     500     {object}  map[string]string
// @Failure     503     {object}  map[string]string "job queue full, nothing was saved"
//
// @Router      /job [get]
// @Router      /job [post]
func (srv *UService) handleJob(c *gin.Context) {
	switch c.Request.Method {
	// "getting" jobs should be treated as "subscribing"
	case http.MethodGet:
		// users see their own jobs only (any user could list everyone's,
		// code and file paths included; frontapp hid it by filtering)
		ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
		if err == nil && !privileged(c, ac) {
			q := c.Request.URL.Query()
			q.Del("uids")
			q.Del("jids")
			q.Set("uid", ac.UID)
			c.Request.URL.RawQuery = q.Encode()
		} else if err != nil && !c.GetBool(ctxPrivileged) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing caller identity"})

			return
		}
		srv.respondJobsQuery(c)
	case http.MethodPost:
		srv.submitJobs(c, true)
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

// handleJobAdmin handles administrative operations on jobs.
//
// @Summary Admin job endpoint
// @Description Query, submit (as root: no input/output permission checks), update and delete jobs.
// @Tags admin, jobs
//
// @Accept json
// @Produce json
//
// @Param uid query int false "Filter jobs by single user ID"
// @Param uids query string false "Comma-separated list of user IDs to filter jobs"
// @Param jids query string false "Comma-separated list of job IDs to retrieve or delete"
// @Param jid query int false "Single job ID to retrieve or delete"
// @Param limit query string false "Pagination limit for job list"
// @Param offset query string false "Pagination offset for job list"
//
// @Param job body ut.Job true "Job object for POST (single) and PUT"
// @Param jobs body []ut.Job true "Job array for POST (multiple)"
//
// @Success 200 {object} map[string]interface{} "Success with job(s) content or status message"
// @Failure 400 {object} map[string]string "Bad request (e.g., parse error)"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 405 {object} map[string]string "Method not allowed"
// @Failure 503 {object} map[string]string "job queue full, nothing was saved"
//
// @Router /admin/job [get]
// @Router /admin/job [post]
// @Router /admin/job [put]
// @Router /admin/job [delete]
func (srv *UService) handleJobAdmin(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		srv.respondJobsQuery(c)
	case http.MethodPost:
		srv.submitJobs(c, false)
	case http.MethodPut:
		var job ut.Job
		if err := c.BindJSON(&job); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind data"})

			return
		}
		if job.JID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "must specify a valid job id"})

			return
		}
		if err := srv.updateJob(c.Request.Context(), job); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update the job"})

			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "update success"})
	case http.MethodDelete:
		srv.deleteJobs(c)
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

// respondJobsQuery answers GET /job and GET /admin/job: jobs by uids, by uid,
// one by jids=<id>, or all (jids empty or "*", with limit/offset).
func (srv *UService) respondJobsQuery(c *gin.Context) {
	if uids := c.Query("uids"); uids != "" {
		uidsInt, err := ut.SplitToInt(uids, ",")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi uids"})

			return
		}
		jobs, err := srv.getJobsByUIDs(c.Request.Context(), uidsInt)
		if err != nil {
			log.Printf("failed to retrieve jobs by uids %v: %v", uidsInt, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve jobs by uid"})

			return
		}
		c.JSON(http.StatusOK, gin.H{"content": jobs})

		return
	}
	if uid := c.Query("uid"); uid != "" {
		uidInt, err := strconv.Atoi(uid)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi uid"})

			return
		}
		jobs, err := srv.getJobsByUID(c.Request.Context(), uidInt)
		if err != nil {
			log.Printf("failed to retrieve jobs by uid %v: %v", uidInt, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve jobs by uid"})

			return
		}
		c.JSON(http.StatusOK, gin.H{"content": jobs})

		return
	}
	jid := c.Query("jids")
	if jid == "" || jid == "*" {
		jobs, err := srv.getAllJobs(c.Request.Context(), c.Query("limit"), c.Query("offset"))
		if err != nil {
			log.Printf("failed to retrieve the jobs: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the jobs"})

			return
		}
		c.JSON(http.StatusOK, gin.H{"content": jobs})

		return
	}
	jidInt, err := strconv.Atoi(jid)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi jid"})

		return
	}
	job, err := srv.getJobByID(c.Request.Context(), jidInt)
	if err != nil {
		log.Printf("failed to retrieve job %d: %v", jidInt, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the job"})

		return
	}
	c.JSON(http.StatusOK, gin.H{"content": job})
}

// parseJobs accepts one job or an array of jobs; single reports which.
func parseJobs(body []byte) ([]ut.Job, bool, error) {
	var job ut.Job
	if err := json.Unmarshal(body, &job); err == nil {
		return []ut.Job{job}, true, nil
	}
	var jobs []ut.Job
	if err := json.Unmarshal(body, &jobs); err != nil {
		return nil, false, err
	}

	return jobs, false, nil
}

// submitJobs saves and queues the posted job(s). With checkIO (user
// submissions) each job's input/output permissions are checked first; admin
// submissions act as root. A job is only kept if it could be queued: on a full
// queue its row is removed again and the client gets 503, so nothing is left
// "pending" forever.
func (srv *UService) submitJobs(c *gin.Context, checkIO bool) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})

		return
	}
	jobs, single, err := parseJobs(body)
	if err != nil || len(jobs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind job(s)"})

		return
	}
	for _, j := range jobs {
		if err := validateJobQuotas(j); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

			return
		}
	}
	if checkIO {
		for _, j := range jobs {
			if err := srv.authorizeJobIO(c, j); err != nil {
				respondJobAuthError(c, err)

				return
			}
		}
	}

	jids := make([]int64, 0, len(jobs))
	for _, job := range jobs {
		// the job's outputs get the submitter's primary group (first in the
		// verified identity); never taken from the request body for users
		if ac, err := BindAccessTarget(c.GetHeader("Access-Target")); checkIO && err == nil {
			job.GID = primaryGID(ac, job.UID)
		} else if job.GID == 0 {
			job.GID = job.UID
		}
		// an output on a group volume belongs to the group (membership was
		// checked above for users)
		if vol, _, ok := strings.Cut(strings.TrimPrefix(job.Output, "/"), "/"); ok {
			if gv, err := srv.fsl.GroupVolume(c.Request.Context(), vol); err == nil {
				job.GID = gv.GID
			}
		}
		jid, err := srv.insertJob(c.Request.Context(), job)
		if err != nil {
			log.Printf("failed to insert the job in the db: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert into db", "published": jids})

			return
		}
		job.JID = jid
		if err := srv.jdp.PublishJob(job); err != nil {
			if rmErr := srv.removeJob(c.Request.Context(), int(jid)); rmErr != nil {
				log.Printf("failed to roll back unqueued job %d: %v", jid, rmErr)
			}
			status := ut.HTTPStatus(err) // queue full or draining: 503
			if status == http.StatusServiceUnavailable {
				c.Header("Retry-After", "30")
			}
			c.JSON(status, gin.H{"error": err.Error(), "published": jids})

			return
		}
		jids = append(jids, jid)
	}

	if single {
		c.JSON(http.StatusOK, gin.H{"status": "job published", "jid": jids[0]})

		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "job(s) published", "jids": jids})
}

// deleteJobs removes jobs by jids=<a,b,c> or jid=<a>.
func (srv *UService) deleteJobs(c *gin.Context) {
	raw := c.Query("jids")
	if raw == "" {
		raw = c.Query("jid")
	}
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide jid(s) as parameter"})

		return
	}
	jids, err := ut.SplitToInt(strings.TrimSpace(raw), ",")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi"})

		return
	}
	if err := srv.removeJobs(c.Request.Context(), jids); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete the job(s)"})

		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "job(s) deleted successfully"})
}

// validateJobQuotas rejects resource values kubernetes can't parse (empty
// ones get defaults at execution time).
func validateJobQuotas(j ut.Job) error {
	for name, v := range map[string]string{
		"memoryRequest": j.MemoryRequest, "memoryLimit": j.MemoryLimit,
		"cpuRequest": j.CPURequest, "cpuLimit": j.CPULimit,
	} {
		if v == "" {
			continue
		}
		if _, err := resource.ParseQuantity(v); err != nil {
			return fmt.Errorf("invalid %s %q", name, v)
		}
	}

	return nil
}

// handleJobLog returns the kept output of a finished (or running) job to its
// owner or root.
//
// @Summary     Job output log
// @Tags        jobs
// @Produce     plain
// @Param       jid query int true "Job ID"
// @Success     200 {string} string "the job's output (tail, up to 64 KiB)"
// @Failure     403 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Router      /job/log [get]
func (srv *UService) handleJobLog(c *gin.Context) {
	jid, err := strconv.Atoi(c.Query("jid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jid must be a number"})

		return
	}
	job, err := srv.getJobByID(c.Request.Context(), jid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})

		return
	}
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil || (!privileged(c, ac) && ac.UID != strconv.FormatInt(job.UID, 10)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not your job"})

		return
	}
	text, err := srv.getJobLog(c.Request.Context(), int64(jid))
	if err != nil {
		log.Printf("failed to read log of job %d: %v", jid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read the job log"})

		return
	}
	c.String(http.StatusOK, text)
}

// handleJobCancel stops a queued or running job (owner or root).
//
// @Summary     Cancel a job
// @Tags        jobs
// @Param       jid query int true "Job ID"
// @Success     200 {object} map[string]string
// @Failure     403 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Failure     409 {object} map[string]string "the job already finished"
// @Router      /job/cancel [post]
func (srv *UService) handleJobCancel(c *gin.Context) {
	jid, err := strconv.Atoi(c.Query("jid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jid must be a number"})

		return
	}
	job, err := srv.getJobByID(c.Request.Context(), jid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})

		return
	}
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil || (!privileged(c, ac) && ac.UID != strconv.FormatInt(job.UID, 10)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not your job"})

		return
	}
	switch job.Status {
	case "completed", "failed", "cancelled", "unknown":
		c.JSON(http.StatusConflict, gin.H{"error": "the job already finished (" + job.Status + ")"})

		return
	}
	if err := srv.jdp.RemoveJob(jid); err != nil {
		log.Printf("failed to cancel job %d: %v", jid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel the job"})

		return
	}
	if job.Status != "running" { // a running job is marked by its executor once stopped
		if err := srv.markJobStatus(c.Request.Context(), int64(jid), "cancelled", 0); err != nil {
			log.Printf("failed to mark job %d cancelled: %v", jid, err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelling", "jid": jid})
}
