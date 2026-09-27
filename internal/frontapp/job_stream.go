package frontendapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

// jobOwner asks uspace (as a service) who owns job jid.
func (srv *HTTPService) jobOwner(ctx context.Context, jid int) (int64, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiServiceURL+"/api/v1/job?jids="+strconv.Itoa(jid), nil)
	if err != nil {
		return 0, http.StatusInternalServerError, err
	}
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, http.StatusBadGateway, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, http.StatusNotFound, fmt.Errorf("uspace answered %s", resp.Status)
	}
	var body struct {
		Content ut.Job `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, http.StatusBadGateway, err
	}

	return body.Content.UID, http.StatusOK, nil
}

// mayWatchJob reports whether the logged-in user owns job jid (admins may
// watch any job).
func (srv *HTTPService) mayWatchJob(c *gin.Context, jid int) (bool, int) {
	if strings.Contains(c.GetString("groups"), "admin") {
		return true, http.StatusOK
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	owner, status, err := srv.jobOwner(ctx, jid)
	if err != nil {
		log.Printf("[ws-ticket] owner of job %d: %v", jid, err)

		return false, status
	}

	return strconv.FormatInt(owner, 10) == fmt.Sprint(c.MustGet("userID")), http.StatusForbidden
}

// handleWSTicket issues a one-minute ticket to watch a job's live output on
// wss (which can't see this session itself).
func (srv *HTTPService) handleWSTicket(c *gin.Context) {
	jid, err := strconv.Atoi(c.Query("jid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jid must be a number"})

		return
	}
	if ok, status := srv.mayWatchJob(c, jid); !ok {
		c.JSON(status, gin.H{"error": "you can only watch your own jobs"})

		return
	}
	ticket := ut.SignWSTicket(srv.Config.ServiceSecretKey, strconv.Itoa(jid), "consumer", fmt.Sprint(c.MustGet("userID")), time.Now())
	c.JSON(http.StatusOK, gin.H{"ticket": ticket, "expiresIn": int(ut.WSTicketTTL.Seconds())})
}

// handleJobLog returns the saved output of one of the user's jobs.
func (srv *HTTPService) handleJobLog(c *gin.Context) {
	jid, err := strconv.Atoi(c.Query("jid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jid must be a number"})

		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiServiceURL+"/api/v1/job/log?jid="+url.QueryEscape(strconv.Itoa(jid)), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})

		return
	}
	// uspace checks ownership against this identity (root for admins)
	who := fmt.Sprintf("%v:%v", c.MustGet("userID"), c.MustGet("groupIDs"))
	if strings.Contains(c.GetString("groups"), "admin") {
		who = "0:0"
	}
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
	req.Header.Set("Access-Target", "0::/ "+who)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "uspace unreachable"})

		return
	}
	defer func() { _ = resp.Body.Close() }()
	c.Status(resp.StatusCode)
	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		log.Printf("failed to relay job log: %v", err)
	}
}
