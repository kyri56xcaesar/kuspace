package userspace

import (
	"encoding/json"
	"io"
	"kyri56xcaesar/myThesis/internal/utils"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

/* HTTP Gin handlers related to Jobs, used by the api to handle endpoints*/

// Api call Handlers
func (srv *UService) HandleJob(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		uids, _ := c.GetQuery("uids")
		if uids != "" {
			// return all jobs from database by uids
			uids_int, err := utils.SplitToInt(uids, ",")
			if err != nil {
				log.Printf("failed to atoi uids: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi uids"})
				return
			}
			jobs, err := srv.dbhJobs.jdh.GetJobsByUids(uids_int)
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
			jobs, err := srv.dbhJobs.jdh.GetAllJobs()
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
		job, err := srv.dbhJobs.jdh.GetJobById(jid_int)
		if err != nil {
			log.Printf("failed to retrieve the job: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the job"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": job})

	case http.MethodPost:
		var job Job
		var jobs []Job
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
			log.Printf("jobs: %v", jobs)
			// handle multiple jobs
			// check for jobs valitidy.

			// "publish" jobs

			// save jobs (insert in DB)

			// respond with status
			return
		}
		log.Printf("job: %v", job)
		// handle single job
		// check for job validity.

		// "publish" job

		// save job (insert in DB)

		// respond with status

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
			uids_int, err := utils.SplitToInt(uids, ",")
			if err != nil {
				log.Printf("failed to atoi uids: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi uids"})
				return
			}
			jobs, err := srv.dbhJobs.jdh.GetJobsByUids(uids_int)
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
			jobs, err := srv.dbhJobs.jdh.GetAllJobs()
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
		job, err := srv.dbhJobs.jdh.GetJobById(jid_int)
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
