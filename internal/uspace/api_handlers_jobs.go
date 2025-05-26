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
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

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
// @Failure     405     {object}  map[string]string
// @Failure     500     {object}  map[string]string
//
// @Router      /job [get]
// @Router      /job [post]
func (srv *UService) handleJob(c *gin.Context) {
	var (
		job  ut.Job
		jobs []ut.Job
	)
	switch c.Request.Method {
	// "getting" jobs should be treated as "subscribing"
	case http.MethodGet:
		limit, _ := c.GetQuery("limit")
		offset, _ := c.GetQuery("offset")
		uids, _ := c.GetQuery("uids")
		if uids != "" {
			// return all jobs from database by uids
			uids_int, err := ut.SplitToInt(uids, ",")
			if err != nil {
				log.Printf("failed to atoi uids: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi uids"})
				return
			}
			jobs, err := srv.getJobsByUids(uids_int)
			if err != nil {
				log.Printf("failed to retrieve jobs by uid: %v, %v", uids_int, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve jobs by uid"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"content": jobs})
			return
		}

		uid, _ := c.GetQuery("uid")
		if uid != "" {
			uid_int, err := strconv.Atoi(uid)
			if err != nil {
				log.Printf("failed to atoi uid: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi uid"})
				return
			}
			jobs, err := srv.getJobsByUid(uid_int)
			if err != nil {
				log.Printf("failed to retrieve jobs by uid: %v, %v", uid_int, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve jobs by uid"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"content": jobs})
			return
		}

		jid, _ := c.GetQuery("jids")
		if jid == "" || jid == "*" {
			// return all jobs from database
			jobs, err := srv.getAllJobs(limit, offset)
			if err != nil {
				log.Printf("failed to retrieve the jobs: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the jobs"})
				return
			}
			// log.Printf("jobs retrieved from db: %+v", jobs)
			c.JSON(http.StatusOK, gin.H{"content": jobs})
			return
		}
		jid_int, err := strconv.Atoi(jid)
		if err != nil {
			log.Printf("failed to atoi jid: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi jid"})
			return
		}
		job, err = srv.getJobById(jid_int)
		if err != nil {
			log.Printf("failed to retrieve the job: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the job"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": job})

	case http.MethodPost:
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("failed to read request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}

		if err = json.Unmarshal(body, &job); err != nil {
			if err = json.Unmarshal(body, &jobs); err != nil {
				log.Printf("failed to bind job(s): %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind job(s)"})
				return
			}
			// check for job validity.s
			// save jobs (insert in DB)
			// also acquire jids
			err := srv.insertJobs(jobs)
			if err != nil {
				log.Printf("failed to save jobs in the db: %+v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert into db"})
				return
			}

			// "publish" jobs
			err = srv.jdp.PublishJobs(jobs)
			if err != nil {
				log.Printf("failed to publish the jobs: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to publish jobs"})
				return
			}

			// respond with status
			c.JSON(http.StatusOK, gin.H{
				"status": "job(s) published",
			})
			return
		}
		// log.Printf("job: %v", job)
		// handle single job
		// check for job validity.

		// save job (insert in DB)
		jid, err := srv.insertJob(job)
		if err != nil {
			log.Printf("failed to insert the job in the db: %+v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert into db"})
			return
		}
		job.Jid = jid
		log.Printf("[Database] Job id acquired: %d", jid)
		// "publish" job
		err = srv.jdp.PublishJob(job)
		if err != nil {
			log.Printf("failed to publish the job: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to publish job"})
			return
		}

		// respond with status
		c.JSON(http.StatusOK, gin.H{
			"status": "job published",
			"jid":    jid,
		})

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "method not allowed",
		})
	}
}

// handleJobAdmin handles administrative operations on jobs.
//
// @Summary Admin job endpoint
// @Description Handles CRUD operations for jobs. Supports multiple HTTP methods.
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
//
// @Router /admin/job [get]
// @Router /admin/job [post]
// @Router /admin/job [put]
// @Router /admin/job [delete]
func (srv *UService) handleJobAdmin(c *gin.Context) {
	var (
		job  ut.Job
		jobs []ut.Job
	)
	switch c.Request.Method {
	case http.MethodGet:
		uids, _ := c.GetQuery("uids")
		if uids != "" {
			// return all jobs from database by uids
			uids_int, err := ut.SplitToInt(uids, ",")
			if err != nil {
				log.Printf("failed to atoi uids: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi uids"})
				return
			}
			jobs, err := srv.getJobsByUids(uids_int)
			if err != nil {
				log.Printf("failed to retrieve jobs by uid: %v, %v", uids_int, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve jobs by uid"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"content": jobs})
			return
		}

		uid, _ := c.GetQuery("uid")
		if uid != "" {
			uid_int, err := strconv.Atoi(uid)
			if err != nil {
				log.Printf("failed to atoi uid: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi uid"})
				return
			}
			jobs, err := srv.getJobsByUid(uid_int)
			if err != nil {
				log.Printf("failed to retrieve jobs by uid: %v, %v", uid_int, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve jobs by uid"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"content": jobs})
			return
		}
		limit, _ := c.GetQuery("limit")
		offset, _ := c.GetQuery("offset")
		jid, _ := c.GetQuery("jids")
		if jid == "" || jid == "*" {
			// return all jobs from database
			jobs, err := srv.getAllJobs(limit, offset)
			if err != nil {
				log.Printf("failed to retrieve the jobs: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the jobs"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"content": jobs})
			return
		}
		jid_int, err := strconv.Atoi(jid)
		if err != nil {
			log.Printf("failed to atoi jid: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi jid"})
			return
		}
		job, err := srv.getJobById(jid_int)
		if err != nil {
			log.Printf("failed to retrieve the job: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the job"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": job})
	case http.MethodPost:
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("failed to read request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}

		if err = json.Unmarshal(body, &job); err != nil {
			if err = json.Unmarshal(body, &jobs); err != nil {
				log.Printf("failed to bind job(s): %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind job(s)"})
				return
			}
			// check for job validity.s
			// save jobs (insert in DB)
			// also acquire jids
			err := srv.insertJobs(jobs)
			if err != nil {
				log.Printf("failed to save jobs in the db: %+v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert into db"})
				return
			}

			// "publish" jobs
			err = srv.jdp.PublishJobs(jobs)
			if err != nil {
				log.Printf("failed to publish the jobs: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to publish jobs"})
				return
			}

			// respond with status
			c.JSON(http.StatusOK, gin.H{
				"status": "job(s) published",
			})
			return
		}
		// log.Printf("job: %v", job)
		// handle single job
		// check for job validity.

		// save job (insert in DB)
		jid, err := srv.insertJob(job)
		if err != nil {
			log.Printf("failed to insert the job in the db: %+v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert into db"})
			return
		}
		job.Jid = jid
		log.Printf("[Database] Job id acquired: %d", jid)
		// "publish" job
		err = srv.jdp.PublishJob(job)
		if err != nil {
			log.Printf("failed to publish the job: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to publish job"})
			return
		}

		// respond with status
		c.JSON(http.StatusOK, gin.H{
			"status": "job published",
			"jid":    jid,
		})

	case http.MethodPut:
		err := c.BindJSON(&job)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind data"})
			return
		}

		if job.Jid == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "must specify a valid job id"})
			return
		}

		err = srv.updateJob(job)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update the job"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "update sucess"})

	case http.MethodDelete:
		jids, _ := c.GetQuery("jids")
		if jids != "" {
			jids_int, err := ut.SplitToInt(strings.TrimSpace(jids), ",")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi"})
				return
			}
			err = srv.removeJobs(jids_int)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete the jobs"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "job(s) deleted successfully"})
			return
		}

		jid, _ := c.GetQuery("jid")
		if jid != "" {
			jid_int, err := strconv.Atoi(strings.TrimSpace(jid))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi"})
				return
			}
			err = srv.removeJob(jid_int)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete the job"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "job deleted successfully"})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide jid(s) as parameter"})
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "method not allowed",
		})
	}
}
