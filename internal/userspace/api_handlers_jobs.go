package userspace

/*
	http api handlers for the userspace service
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

	ut "kyri56xcaesar/myThesis/internal/utils"

	"github.com/gin-gonic/gin"
)

// Api call Handlers
func (srv *UService) HandleJob(c *gin.Context) {
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
			jobs, err := srv.GetJobsByUids(uids_int)
			if err != nil {
				log.Printf("failed to retrieve jobs by uid: %v, %v", uids_int, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve jobs by uid"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"content": jobs})
			return
		}

		jid, _ := c.GetQuery("jids")
		if jid == "" || jid == "*" {
			// return all jobs from database
			jobs, err := srv.GetAllJobs(limit, offset)
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
		job, err := srv.GetJobById(jid_int)
		if err != nil {
			log.Printf("failed to retrieve the job: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the job"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": job})

	case http.MethodPost:
		var job ut.Job
		var jobs []ut.Job
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("failed to read request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}
		log.Printf("request body: %s", string(body))

		if err = json.Unmarshal(body, &job); err != nil {
			if err = json.Unmarshal(body, &jobs); err != nil {
				log.Printf("failed to bind job(s): %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind job(s)"})
				return
			}
			// handle multiple jobs
			// check for jobs valitidy.

			// save jobs (insert in DB)
			srv.InsertJobs(jobs)

			// "publish" jobs
			srv.jdp.PublishJobs(jobs)

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
		jid, err := srv.InsertJob(job)
		if err != nil {
			log.Printf("failed to insert the job in the db: %+v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert into db"})
			return
		}
		job.Jid = int(jid)
		log.Printf("jid acquired: %d", jid)
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

func (srv *UService) HandleJobAdmin(c *gin.Context) {
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
			jobs, err := srv.GetJobsByUids(uids_int)
			if err != nil {
				log.Printf("failed to retrieve jobs by uid: %v, %v", uids_int, err)
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
			jobs, err := srv.GetAllJobs(limit, offset)
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
		job, err := srv.GetJobById(jid_int)
		if err != nil {
			log.Printf("failed to retrieve the job: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the job"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": job})

	case http.MethodPost:
	case http.MethodPut:
	case http.MethodDelete:
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "method not allowed",
		})
	}
}
