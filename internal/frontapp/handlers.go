package frontendapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

/*
	Bunch of handlers for the frontend app

	fetching data from other Apis and rendering them in the given format
default: rendered html template
*/

/* other apis addresses
*
* init by configuration on startup...
* */
var (
	authServiceURL string
	authVersion    string = "/v1"

	apiServiceURL string
)

/*
*********************************************************************
*   Users
* */

/*
 * function handler (GET requests) that responds with all the existing users
 *
 * -> format specified (default rendered html template)
 * @TODO: allow range request (paging)
 */
func (srv *HTTPService) handleFetchUsers(c *gin.Context) {
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	req, err := http.NewRequest(http.MethodGet, authServiceURL+authVersion+"/admin/users", nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch users"})
		return
	}
	defer response.Body.Close()

	var resp struct {
		Content []ut.User `json:"content"`
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	err = json.Unmarshal(body, &resp)
	if err != nil {
		log.Printf("failed to unmarshal response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	// sort users on uid
	sort.Slice(resp.Content, func(i, j int) bool {
		return resp.Content[i].Uid < resp.Content[j].Uid
	})

	// answer according to format
	format := c.Request.URL.Query().Get("format")

	// Render the HTML template
	respondInFormat(c, format, resp.Content, "users_template.html")
}

/*
 * a handler on an admin useradd call (similar to register but with less strictness)
 *
 *
 */
func (srv *HTTPService) handleUseradd(c *gin.Context) {
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var uac UseraddClaim
	if err := c.ShouldBind(&uac); err != nil {
		log.Printf("Register binding error: %v", err)
		// Respond with the appropriate error on the template.
		c.JSON(http.StatusBadRequest, gin.H{"error": "Register binding"})
		return
	}

	// Forward login request to the auth service
	resp, err := jsonPostRequest(authServiceURL+authVersion+"/admin/useradd", accessToken, gin.H{
		"user": ut.User{
			Username: uac.Username,
			Info:     uac.Email,
			Home:     "/home/" + uac.Username,
			Shell:    "gshell",
			Password: ut.Password{
				Hashpass: uac.Password,
			},
		},
	})
	if err != nil {
		log.Printf("Error forwarding register request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "forwarding fail"})
		return

	}
	defer resp.Body.Close()

	// Check the response status from the auth service
	if resp.StatusCode != http.StatusOK {
		log.Printf("Auth service returned status: %v", resp.Status)
		var ErrResp struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ErrResp); err != nil {
			log.Printf("Error decoding auth err response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "serious"})
			return
		}

		c.JSON(resp.StatusCode, ErrResp)
		return
	}

	var useraddResp RegResponse
	err = json.NewDecoder(resp.Body).Decode(&useraddResp)
	if err != nil {
		log.Printf("failed to decode resp json body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "user added"})
}

/* Forward a user deletion request to the AUTH service
 *
 * perform some authoriaztion checks meanwhile...
 *
 * return if deletion succeeded or not.
 */
func (srv *HTTPService) handleUserdel(c *gin.Context) {
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid := c.Request.URL.Query().Get("uid")
	if uid == "" {
		log.Printf("missing uid parameter, must provide...")
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing uid param"})
		return
	}
	req, err := http.NewRequest(http.MethodDelete, authServiceURL+authVersion+"/admin/userdel?uid="+uid, nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to delete user"})
		return
	}
	defer response.Body.Close()

	var resp struct {
		Message string `json:"message"`
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	err = json.Unmarshal(body, &resp)
	if err != nil {
		log.Printf("failed to unmarshal response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	// On success we should delete the user/group volume as well...

	// Render the HTML template
	c.String(response.StatusCode, "%v", resp.Message)
}

func (srv *HTTPService) handleUserpatch(c *gin.Context) {
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var rq struct {
		Username string `json:"username" form:"username"`
		Password string `json:"password" form:"password"`
		Email    string `json:"info" form:"info"`
		Home     string `json:"home" form:"home"`
		Shell    string `json:"shell" form:"shell"`
		Gid      string `json:"pgroup" form:"pgroup"`
		Groups   string `json:"groups" form:"groups"`
		Uid      int    `json:"uid" form:"uid"`
	}

	if err := c.ShouldBind(&rq); err != nil {
		log.Printf("binding error: %v", err)
		// Respond with the appropriate error on the template.
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad binding"})
		return
	}

	jsonRq, err := json.Marshal(&rq)
	if err != nil {
		log.Printf("error marshalling req body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error marshalling"})
		return
	}

	req, err := http.NewRequest(http.MethodPatch, authServiceURL+authVersion+"/admin/userpatch", bytes.NewBuffer(jsonRq))
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to delete user"})
		return
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		log.Printf("request failed with code: %v", response.Status)
		c.JSON(response.StatusCode, gin.H{"error": "patch failed"})
		return
	}

	var resp struct {
		Message string `json:"message"`
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	err = json.Unmarshal(body, &resp)
	if err != nil {
		log.Printf("failed to unmarshal response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	// Render the HTML template
	c.String(http.StatusOK, "%v", resp.Message)
}

/*
*********************************************************************
*   Groups
* */
func (srv *HTTPService) handleFetchGroups(c *gin.Context) {
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	req, err := http.NewRequest(http.MethodGet, authServiceURL+authVersion+"/admin/groups", nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch users"})
		return
	}
	defer response.Body.Close()

	var resp struct {
		Content []ut.Group `json:"content"`
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	err = json.Unmarshal(body, &resp)
	if err != nil {
		log.Printf("failed to unmarshal response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	sort.Slice(resp.Content, func(i, j int) bool {
		return resp.Content[i].Gid < resp.Content[j].Gid
	})

	// Render the HTML template

	format := c.Request.URL.Query().Get("format")

	// Render the HTML template
	respondInFormat(c, format, resp.Content, "groups_template.html")
}

func (srv *HTTPService) handleGroupadd(c *gin.Context) {
	/* Since, this is an admin function, verify early that access token exists
	* perhaps, unecessary.
	* */
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Groupname string `json:"groupname" form:"groupname"`
	}

	if err := c.ShouldBind(&req); err != nil {
		log.Printf("failed to bind request: %v", err)
		c.JSON(400, gin.H{"error": "bad binding"})
		return
	}

	/* forward login request to the auth service */
	resp, err := jsonPostRequest(authServiceURL+authVersion+"/admin/groupadd", accessToken, req)
	if err != nil {
		log.Printf("Error forwarding register request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "forwarding fail"})
		return

	}
	defer resp.Body.Close()

	// Check the response status from the auth service
	if resp.StatusCode != http.StatusOK {
		log.Printf("Auth service returned status: %v", resp.Status)
		var ErrResp struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ErrResp); err != nil {
			log.Printf("Error decoding auth err response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "serious"})
			return
		}

		c.JSON(resp.StatusCode, ErrResp)
		return
	}
}

func (srv *HTTPService) handleGroupdel(c *gin.Context) {
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	gid := c.Request.URL.Query().Get("gid")
	if gid == "" {
		log.Printf("missing gid parameter, must provide...")
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing gid param"})
		return
	}
	req, err := http.NewRequest(http.MethodDelete, authServiceURL+authVersion+"/admin/groupdel?gid="+gid, nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to delete group"})
		return
	}
	defer response.Body.Close()

	var resp struct {
		Message string `json:"message"`
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	err = json.Unmarshal(body, &resp)
	if err != nil {
		log.Printf("failed to unmarshal response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	c.String(response.StatusCode, "%v", resp.Message)
}

func (srv *HTTPService) handleGrouppatch(c *gin.Context) {
}

/*
*********************************************************************
*   Resources
* */
func (srv *HTTPService) handleFetchResources(c *gin.Context) {
	_, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	volume := c.Query("volume")
	if volume == "" {
		volume = "*"
	}
	struc_type := c.DefaultQuery("struct", "list")

	req, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/resources?struct="+struc_type, nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	req.Header.Set("Access-Target", fmt.Sprintf(":%s:/ 0:0", volume))
	req.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))
	// req.Header.Set("Authorization", "Bearer "+acc)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch users"})
		return
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		log.Printf("resource not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Resource not found"})
		return
	}

	if response.StatusCode >= 300 {
		c.Status(http.StatusBadGateway)
		_, err = io.Copy(c.Writer, response.Body)
		if err != nil {
			log.Printf("failed to write response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write response"})
			return
		}
		return
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	switch struc_type {
	case "tree":
		var data map[string]any
		if err := json.Unmarshal(body, &data); err != nil {
			log.Printf("failed to unmarshal response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
			return
		}

		tree := parseTreeNode("/", data)
		if tree == nil {
			log.Printf("failed to build tree struct")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build tree struct"})
			return
		}

		c.HTML(http.StatusOK, "tree-resources.html", tree)
	default:
		var data []ut.Resource
		err = json.Unmarshal(body, &data)
		if err != nil {
			log.Printf("failed to unmarshal response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
			return
		}
		format := c.Request.URL.Query().Get("format")
		respondInFormat(c, format, data, "list-resources.html")

	}
}

func (srv *HTTPService) handleResourceUpload(c *gin.Context) {
	uid, _ := c.Get("user_id")
	group_ids, _ := c.Get("group_ids")
	volume := c.Query("volume")
	if volume == "" {
		volume = c.Request.Header.Get("X-Volume-Target")
		if volume == "" {
			volume = srv.Config.MINIO_DEFAULT_BUCKET
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiServiceURL+"/api/v1/resource/upload", c.Request.Body)
	if err != nil {
		log.Printf("failed to create a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// forward headers
	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	req.Header.Set("Access-Target", fmt.Sprintf("0:%s:/ %v:%v", volume, uid, group_ids))
	req.Header.Set("Authorization", c.Request.Header.Get("Authorization"))
	req.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to upload resource"})
		return
	}
	defer resp.Body.Close()

	// propagate response headers and status
	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	c.Status(resp.StatusCode)

	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		log.Printf("failed to write response body: %v", err)
	}
}

func (srv *HTTPService) handleResourceDownload(c *gin.Context) {
	uid, _ := c.Get("user_id")
	group_ids, _ := c.Get("group_ids")
	fpath := c.Request.URL.Query().Get("target")

	if fpath == "" {
		log.Printf("must provide a target")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a target"})
		return
	}

	req, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/resource/download", c.Request.Body)
	if err != nil {
		log.Printf("failed to create a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	volume := c.Query("volume")
	if volume == "" {
		volume = srv.Config.MINIO_DEFAULT_BUCKET
	}

	req.Header.Add("Access-Target", fmt.Sprintf(":%s:%v %v:%v", volume, fpath, uid, group_ids))
	req.Header.Add("Authorization", c.Request.Header.Get("Authorization"))
	req.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

	log.Printf("%v %v:%v", fpath, uid, group_ids)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to upload resource"})
		return
	}

	defer response.Body.Close()
	c.Status(response.StatusCode)
	for key, values := range response.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	downloadName := filepath.Base(fpath)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadName))
	_, err = io.Copy(c.Writer, response.Body)
	if err != nil {
		log.Printf("failed to write response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write response"})
	}
}

func (srv *HTTPService) handleResourcePreview(c *gin.Context) {
	whoami, exists := c.Get("user_id")
	groups, gexists := c.Get("group_ids")
	if !exists || !gexists {
		log.Printf("uid or gids don't exist... bad authentication")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bad auth"})
		return
	}
	rid := c.Request.URL.Query().Get("rid")
	r_name := c.Request.URL.Query().Get("resourcename")
	if r_name == "" || rid == "" {
		log.Printf("must provide resource name")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide resource name"})
		return
	}

	req, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/resource/preview?rid="+rid, c.Request.Body)
	if err != nil {
		log.Printf("failed to create a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create a http request"})
		return
	}
	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	volume := c.Query("volume")
	if volume == "" {
		volume = srv.Config.MINIO_DEFAULT_BUCKET
	}

	req.Header.Add("Access-Target", fmt.Sprintf(":%s:%v %v:%v", volume, r_name, whoami, groups))
	req.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("request error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed request"})
		return
	}

	defer response.Body.Close()

	c.Status(response.StatusCode)
	for key, values := range response.Header {
		for _, value := range values {
			if key != "Content-Length" {
				c.Header(key, value)
			}
		}
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed rq"})
		return
	}

	format := c.Request.URL.Query().Get("format")

	// respond with a rendered template with the data
	respondInFormat(c, format, string(body), "resource-preview.html")
}

func (srv *HTTPService) handleResourceMove(c *gin.Context) {

	r_name := c.Query("resourcename")
	if r_name == "" {
		log.Printf("must provide resource name")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide resource name"})
		return
	}

	volume := c.Query("volume")
	if volume == "" {
		volume = srv.Config.MINIO_DEFAULT_BUCKET
	}

	newName := c.PostForm("resourcename")
	if err := ut.ValidateObjectName(newName); err != nil { // should check for name validity as well!
		log.Printf("invalid name: %s: %v", newName, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "should provide a proper new name"})
		return
	}

	req, err := http.NewRequest(http.MethodPatch, apiServiceURL+"/api/v1/resource/mv?dest="+volume+"/"+newName, nil)
	if err != nil {
		log.Printf("error creating a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failure"})
		return
	}

	user_id, exists := c.Get("user_id")
	gids, gexists := c.Get("group_ids")
	if !exists || !gexists {
		log.Printf("uid or gids don't exist... bad authentication")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bad auth"})
		return
	}

	req.Header.Set("Access-Target", fmt.Sprintf(":%s:%v %v:%v", volume, r_name, user_id, gids))
	req.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to upload resource"})
		return
	}

	defer response.Body.Close()

	c.Status(response.StatusCode)
	for key, values := range response.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	_, err = io.Copy(c.Writer, response.Body)
	if err != nil {
		log.Printf("failed to write response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write response"})
	}
}

func (srv *HTTPService) handleResourceDelete(c *gin.Context) {
	// will be confgured for multiple deletion
	resource_target := c.Request.URL.Query().Get("name")
	if resource_target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing target parameter"})
		return
	}
	uid, _ := c.Get("user_id")
	group_ids, _ := c.Get("group_ids")

	req, err := http.NewRequest(http.MethodDelete, apiServiceURL+"/api/v1/resource/rm", c.Request.Body)
	if err != nil {
		log.Printf("failed to create a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	volume := c.Query("volume")
	if volume == "" {
		volume = srv.Config.MINIO_DEFAULT_BUCKET
	}

	req.Header.Add("Access-Target", fmt.Sprintf(":%s:%s %v:%v", volume, resource_target, uid, group_ids))
	req.Header.Add("Authorization", c.Request.Header.Get("Authorization"))
	req.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to upload resource"})
		return
	}

	defer response.Body.Close()

	c.Status(response.StatusCode)
	for key, values := range response.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	_, err = io.Copy(c.Writer, response.Body)
	if err != nil {
		log.Printf("failed to write response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write response"})
	}
}

func (srv *HTTPService) handleResourceCopy(c *gin.Context) {
	access_token, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("failed to retrieve access token cookie: %v", err)
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
		return
	}

	rid := c.Request.URL.Query().Get("rid")

	r_name := c.Request.URL.Query().Get("resource")
	if r_name == "" || rid == "" {
		log.Printf("must provide resource name")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide resource name"})
		return
	}

	// should filter input
	//if err := utils.allowedFileName(filename); err != nil {
	//log.Printf("bad resource name: %v", err)
	//c.JSON(http.StatusBadRequest, gin.H{"error":"bad resource name"})
	//return
	//}

	req, err := http.NewRequest(http.MethodPatch, apiServiceURL+"/api/v1/resource/cp?rid="+rid, c.Request.Body)
	if err != nil {
		log.Printf("error creating a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failure"})
		return
	}

	user_id, exists := c.Get("user_id")
	gids, gexists := c.Get("group_ids")
	if !exists || !gexists {
		log.Printf("uid or gids don't exist... bad authentication")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bad auth"})
		return
	}
	volume := c.Query("volume")
	if volume == "" {
		volume = srv.Config.MINIO_DEFAULT_BUCKET
	}

	req.Header.Add("Authorization", "Bearer "+access_token)
	req.Header.Add("Access-Target", fmt.Sprintf(":%s:%v %v:%v", volume, r_name, user_id, gids))
	req.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to upload resource"})
		return
	}

	defer response.Body.Close()

	c.Status(response.StatusCode)
	for key, values := range response.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	_, err = io.Copy(c.Writer, response.Body)
	if err != nil {
		log.Printf("failed to write response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write response"})
	}
}

func (srv *HTTPService) handleResourcePerms(c *gin.Context) {
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var (
		req      *http.Request
		endpoint string
	)

	owner := c.PostForm("owner")
	group := c.PostForm("group")
	perms := c.PostForm("permissions")

	log.Printf("Received Form Values - Owner: %s, Group: %s, Permissions: %s", owner, group, perms)
	formData := url.Values{}

	switch {
	case owner != "":
		endpoint = "/api/v1/resource/ownership"
		formData.Set("owner", owner)
	case group != "":
		endpoint = "/api/v1/resource/group"
		formData.Set("group", group)
	case perms != "":
		endpoint = "/api/v1/resource/permissions"
		formData.Set("permissions", perms)
	default:
		log.Printf("No valid form field provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid form field provided"})
		return
	}
	rid := c.Request.URL.Query().Get("rid")
	if rid == "" {
		log.Printf("rid empty: must provide a rid")
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty rid, must provide"})
		return
	}
	endpoint += "?rid=" + rid

	requestBody := strings.NewReader(formData.Encode())
	req, err = http.NewRequest(http.MethodPatch, apiServiceURL+endpoint, requestBody)
	if err != nil {
		log.Printf("failed to create a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create a new request"})
		return
	}

	whoami, exists := c.Get("user_id")
	mygroups, gexists := c.Get("groups")
	if !exists || !gexists {
		log.Printf("uid or groups were not set correctly. Authencitation fail")
		c.JSON(http.StatusInsufficientStorage, gin.H{"error": "failed auth"})
		return
	}
	volume := c.Query("volume")
	if volume == "" {
		volume = srv.Config.MINIO_DEFAULT_BUCKET
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Access-Target", fmt.Sprintf(":%s:$rids=%s %v:%v", volume, rid, whoami, mygroups))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to upload resource"})
		return
	}

	defer response.Body.Close()

	c.Status(response.StatusCode)
	for key, values := range response.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	_, err = io.Copy(c.Writer, response.Body)
	if err != nil {
		log.Printf("failed to write response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write response"})
	}
}

/*
*********************************************************************
*   Volumes
* */
func (srv *HTTPService) handleFetchVolumes(c *gin.Context) {
	volumeReq, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/admin/volumes", nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var volumeResp struct {
		Content []ut.Volume `json:"content"`
	}

	response, err := client.Do(volumeReq)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch jobs"})
		return
	}
	defer response.Body.Close()

	if err := json.NewDecoder(response.Body).Decode(&volumeResp); err != nil {
		log.Printf("volume request error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch volumes"})
		return
	}
	combinedData := gin.H{
		"volumes": volumeResp.Content,
	}

	format := c.Request.URL.Query().Get("format")

	// Render the HTML template
	respondInFormat(c, format, combinedData, "volumes_template.html")
}

func (srv *HTTPService) handleVolumeadd(c *gin.Context) {
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req ut.Volume
	if err := c.ShouldBind(&req); err != nil {
		log.Printf("failed to bind request: %v", err)
		c.JSON(400, gin.H{"error": "bad binding"})
		return
	}

	/* forward login request to the auth service */
	resp, err := jsonPostRequest(apiServiceURL+"/api/v1/admin/volumes", accessToken, req)
	if err != nil {
		log.Printf("Error forwarding register request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "forwarding fail"})
		return

	}
	defer resp.Body.Close()

	// Check the response status from the auth service
	if resp.StatusCode != http.StatusOK {
		var ErrResp struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ErrResp); err != nil {
			log.Printf("Error decoding auth err response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "serious"})
			return
		}

		c.JSON(resp.StatusCode, ErrResp)
		return
	}
}

func (srv *HTTPService) handleVolumedel(c *gin.Context) {
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	vname := c.Query("volume")
	if vname == "" {
		log.Printf("missing vname parameter, must provide...")
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing vname param"})
		return
	}
	req, err := http.NewRequest(http.MethodDelete, apiServiceURL+"/api/v1/admin/volumes?volume="+vname, nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to delete user"})
		return
	}
	defer response.Body.Close()

	var resp struct {
		Message string `json:"message"`
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	err = json.Unmarshal(body, &resp)
	if err != nil {
		log.Printf("failed to unmarshal response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	c.String(response.StatusCode, "%v", resp.Message)
}

/*
*********************************************************************
*   Jobs
* */
const customTimeLayout = "2006-01-02 15:04:05-07:00" // Match your format
func (srv *HTTPService) jobsHandler(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		jobReq, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/job", nil)
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		jobReq.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))
		jobReq.Header.Set("Access-Target", "0::/ 0:0")

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(jobReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch jobs"})
			return
		}
		defer response.Body.Close()

		var jobResp struct {
			Content []ut.Job `json:"content"`
		}

		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
			return
		}

		err = json.Unmarshal(body, &jobResp)
		if err != nil {
			log.Printf("failed to unmarshal response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
			return
		}

		// sort on given argument
		sortBy := c.Request.URL.Query().Get("sort")

		switch sortBy {
		case "output":
			sort.Slice(jobResp.Content, func(i, j int) bool {
				return jobResp.Content[i].Output > jobResp.Content[j].Output
			})
		case "uid":
			sort.Slice(jobResp.Content, func(i, j int) bool {
				return jobResp.Content[i].Uid > jobResp.Content[j].Uid
			})
		case "jid":
			sort.Slice(jobResp.Content, func(i, j int) bool {
				return jobResp.Content[i].Jid > jobResp.Content[j].Jid
			})
		case "status":
			sort.Slice(jobResp.Content, func(i, j int) bool {
				return compareStatus(jobResp.Content[i].Status, jobResp.Content[j].Status)
			})
		case "created_at", "time":
			// sort users on time
			sort.Slice(jobResp.Content, func(i, j int) bool {
				t1, err1 := time.Parse(customTimeLayout, jobResp.Content[i].Created_at)
				t2, err2 := time.Parse(customTimeLayout, jobResp.Content[j].Created_at)

				// Handle parsing errors gracefully (e.g., keep original order)
				if err1 != nil || err2 != nil {
					return false
				}

				return t1.After(t2)
			})
		default:
			// sort users on time
			sort.Slice(jobResp.Content, func(i, j int) bool {
				t1, err1 := time.Parse(customTimeLayout, jobResp.Content[i].Created_at)
				t2, err2 := time.Parse(customTimeLayout, jobResp.Content[j].Created_at)

				// Handle parsing errors gracefully (e.g., keep original order)
				if err1 != nil || err2 != nil {
					return false
				}

				return t1.After(t2)
			})
		}

		// answer according to format
		format := c.Request.URL.Query().Get("format")

		// Render the HTML template
		respondInFormat(c, format, jobResp.Content, "jobs_list_template.html")
	case http.MethodPost:
		// lets fix the uid (identify ourselves)
		var job ut.Job
		err := c.ShouldBind(&job)
		if err != nil {
			log.Printf("failed to bind json body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind"})
			return
		}

		err = job.ValidateForm(srv.Config.J_MAX_CPU, srv.Config.J_MAX_MEM, srv.Config.J_MAX_STORAGE, int64(srv.Config.J_MAX_PARALLELISM), srv.Config.J_MAX_TIMEOUT, srv.Config.J_MAX_LOGIC_CHARS)
		if err != nil {
			log.Printf("failed to validate form: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// our uid
		uid, exists := c.Get("user_id")
		if !exists {
			log.Printf("uid not set correctly... should be unreachable")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "incosiderable"})
			return
		}
		job.Uid, err = strconv.Atoi(uid.(string))
		if err != nil {
			log.Printf("failed to atoi uid value: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to atoi uid"})
			return
		}
		job_json, err := json.Marshal(job)
		if err != nil {
			log.Printf("failed to marshal job: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal job"})
			return
		}
		jobReq, err := http.NewRequest(http.MethodPost, apiServiceURL+"/api/v1/job", bytes.NewBuffer(job_json))
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		jobReq.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(jobReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch jobs"})
			return
		}

		var resp struct {
			Jid    int    `json:"jid"`
			Status string `json:"status"`
		}

		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
			return
		}
		defer response.Body.Close()

		err = json.Unmarshal(body, &resp)
		if err != nil {
			log.Printf("failed to unmarshal response body: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to retrieve expected response"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"jid": resp.Jid, "status": resp.Status, "output": job.Output})

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not supported"})

	}

}

func (srv *HTTPService) appsHandler(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		appReq, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/app", nil)
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		appReq.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(appReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch data"})
			return
		}
		defer response.Body.Close()

		var resp struct {
			Content []ut.Application `json:"content"`
		}

		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
			return
		}

		err = json.Unmarshal(body, &resp)
		if err != nil {
			log.Printf("failed to unmarshal response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
			return
		}

		// sort on given argument
		sortBy := c.Request.URL.Query().Get("sort")

		switch sortBy {
		case "name":
			sort.Slice(resp.Content, func(i, j int) bool {
				return resp.Content[i].Name > resp.Content[j].Name
			})
		case "image":
			sort.Slice(resp.Content, func(i, j int) bool {
				return resp.Content[i].Image > resp.Content[j].Image
			})
		case "version":
			sort.Slice(resp.Content, func(i, j int) bool {
				return resp.Content[i].Version > resp.Content[j].Version
			})
		case "status":
			sort.Slice(resp.Content, func(i, j int) bool {
				return compareStatus(resp.Content[i].Status, resp.Content[j].Status)
			})
		case "author":
			sort.Slice(resp.Content, func(i, j int) bool {
				return resp.Content[i].Author > resp.Content[j].Author
			})
		case "author_id":
			sort.Slice(resp.Content, func(i, j int) bool {
				return resp.Content[i].AuthorId > resp.Content[j].AuthorId
			})
		case "created_at", "time":
			// sort users on time
			sort.Slice(resp.Content, func(i, j int) bool {
				t1, err1 := time.Parse(customTimeLayout, resp.Content[i].CreatedAt)
				t2, err2 := time.Parse(customTimeLayout, resp.Content[j].CreatedAt)

				// Handle parsing errors gracefully (e.g., keep original order)
				if err1 != nil || err2 != nil {
					return false
				}

				return t1.After(t2)
			})
		case "inserted_at":
			// sort users on time
			sort.Slice(resp.Content, func(i, j int) bool {
				t1, err1 := time.Parse(customTimeLayout, resp.Content[i].InsertedAt)
				t2, err2 := time.Parse(customTimeLayout, resp.Content[j].InsertedAt)

				// Handle parsing errors gracefully (e.g., keep original order)
				if err1 != nil || err2 != nil {
					return false
				}

				return t1.After(t2)
			})

		}

		// answer according to format
		format := c.Request.URL.Query().Get("format")

		// Render the HTML template
		respondInFormat(c, format, resp.Content, "apps_list_template.html")
	case http.MethodPost:

		// lets fix the uid (identify ourselves)
		var job ut.Job
		err := c.ShouldBindJSON(&job)
		if err != nil {
			log.Printf("failed to bind json body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind"})
			return
		}

		// log.Printf("%+v", job)
		// our uid
		uid, exists := c.Get("user_id")
		if !exists {
			log.Printf("uid not set correctly... should be unreachable")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "incosiderable"})
			return
		}
		job.Uid, err = strconv.Atoi(uid.(string))
		if err != nil {
			log.Printf("failed to atoi uid value: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to atoi uid"})
			return
		}
		job_json, err := json.Marshal(job)
		if err != nil {
			log.Printf("failed to marshal job: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal job"})
			return
		}
		jobReq, err := http.NewRequest(http.MethodPost, apiServiceURL+"/api/v1/job", bytes.NewBuffer(job_json))
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		jobReq.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(jobReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch jobs"})
			return
		}
		defer response.Body.Close()
		c.Status(response.StatusCode)
		for key, values := range response.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}

		_, err = io.Copy(c.Writer, response.Body)
		if err != nil {
			log.Printf("failed to write response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write response"})
		}
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not supported"})

	}

}

/*
*********************************************************************
*   Apps
* */

/*
*********************************************************************
*   Partial HTML
* */
func (srv *HTTPService) handleLogin(c *gin.Context) {
	// only on success redirect
	var login LoginRequest

	if err := c.ShouldBind(&login); err != nil {
		log.Printf("Login binding error: %v", err)
		// Respond with the appropriate error on the template.
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad binding"})
		return
	}

	// Forward login request to the auth service
	resp, err := jsonPostRequest(authServiceURL+authVersion+"/login", "", login)
	if err != nil {
		log.Printf("Error forwarding login request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "forwarding fail"})
		return

	}
	defer resp.Body.Close()

	// Check the response status from the auth service
	if resp.StatusCode != http.StatusOK {
		log.Printf("Auth service returned status: %v", resp.Status)
		var ErrResp struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ErrResp); err != nil {
			log.Printf("Error decoding auth err response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "serious"})
			return
		}

		c.JSON(resp.StatusCode, ErrResp)
		return
	}

	// Parse the response from the auth service
	var authResponse struct {
		AccessToken string  `json:"access_token"`
		User        ut.User `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
		log.Printf("Error decoding auth service response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serious"})
		return
	}

	c.SetCookie("access_token", authResponse.AccessToken, 3600, "/api/v1/", "", false, true)

	c.Redirect(http.StatusSeeOther, "/api/v1/verified/admin-panel?info="+authResponse.User.Info)

}

func (srv *HTTPService) handleRegister(c *gin.Context) {
	/*
		Register automatically creates a usergroup after registering the user.
	*/
	var reg RegisterRequest

	if err := c.ShouldBind(&reg); err != nil {
		log.Printf("Register binding error: %v", err)
		// Respond with the appropriate error on the template.
		c.JSON(http.StatusBadRequest, gin.H{"error": "Register binding"})
		return
	}

	// verify password repeat
	if reg.Password != reg.RepeatPassword {
		log.Printf("%v!=%v, password-repeat should match!", reg.Password, reg.RepeatPassword)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Passwords should match"})
		return
	}

	data := ut.User{
		Username: reg.Username,
		Info:     reg.Email,
		Home:     "/home/" + reg.Username,
		Shell:    "gshell",
		Password: ut.Password{
			Hashpass: reg.Password,
		},
	}
	// Forward login request to the auth service
	resp, err := jsonPostRequest(authServiceURL+authVersion+"/register", "", gin.H{
		"user": data,
	})
	if err != nil {
		log.Printf("Error forwarding register request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "forwarding fail"})
		return

	}
	defer resp.Body.Close()

	// Check the response status from the auth service
	if resp.StatusCode != http.StatusOK {
		log.Printf("Auth service returned status: %v", resp.Status)
		var ErrResp struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ErrResp); err != nil {
			log.Printf("Error decoding auth err response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "serious"})
			return
		}

		c.JSON(resp.StatusCode, ErrResp)
		return
	}
	// parse response.
	var authResponse RegResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
		log.Printf("Error decoding auth service response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serious"})
		return
	}
	/* give user's primary group some of the pie..*/
	go func() {
		json_data, err := json.Marshal(ut.GroupVolume{Vid: 1, Gid: authResponse.Pgroup})
		if err != nil {
			log.Printf("failed to marshal to json: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set uv"})
			return
		}
		req, err := http.NewRequest(http.MethodPost, apiServiceURL+"/api/v1/admin/group/volume", bytes.NewBuffer(json_data))
		if err != nil {
			log.Printf("failed to create a new request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set gv"})
			req.Header.Add("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))
			return
		}
		req.Header.Add("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

		_, err = http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("failed to send user group volume claim request: %v", err)
			return
		}

	}()
	/* let user some of the volume pie*/
	go func() {
		json_data, err := json.Marshal(ut.UserVolume{Vid: 1, Uid: authResponse.Uid})
		if err != nil {
			log.Printf("failed to marshal to json: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set uv"})
			return
		}
		req, err := http.NewRequest(http.MethodPost, apiServiceURL+"/api/v1/admin/user/volume", bytes.NewBuffer(json_data))
		if err != nil {
			log.Printf("failed to create a new request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set uv"})
			req.Header.Add("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))
			return
		}
		req.Header.Add("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

		_, err = http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("failed to send user volume claim request: %v", err)
			return
		}

	}()
	// at this point registration should be successful, we can directly login the user
	// .. somehow

	// perform login idk bout htis
	// nvm
	c.Redirect(303, "/api/v1/login")
}

func (srv *HTTPService) editFormHandler(c *gin.Context) {
	volume := c.Query("volume")
	resourcename := c.Query("resourcename")
	rid := c.Query("rid")
	owner := c.Query("owner")
	group := c.Query("group")
	perms := c.Query("perms")
	mygroups, exists := c.Get("groups")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "user groups were not set correctly"})
		return
	}

	var (
		groupsResp struct {
			Content []ut.Group `json:"content"`
		}
		usersResp struct {
			Content []ut.User `json:"content"`
		}
		admin bool
	)

	if strings.Contains(mygroups.(string), "admin") {
		admin = true
	}

	if resourcename == "" || owner == "" || group == "" || perms == "" || rid == "" {
		log.Printf("must provide args")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide information"})
		return
	}

	if admin {
		accessToken, err := c.Cookie("access_token")
		if err != nil {
			log.Printf("missing access_token cookie: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		usersReq, err := http.NewRequest(http.MethodGet, authServiceURL+authVersion+"/admin/users", nil)
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		usersReq.Header.Set("Authorization", "Bearer "+accessToken)

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(usersReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch users"})
			return
		}
		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
			return
		}

		err = json.Unmarshal(body, &usersResp)
		if err != nil {
			log.Printf("failed to unmarshal response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
			return
		}

		groupsReq, err := http.NewRequest(http.MethodGet, authServiceURL+authVersion+"/admin/groups", nil)
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		groupsReq.Header.Set("Authorization", "Bearer "+accessToken)

		response, err = client.Do(groupsReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch users"})
			return
		}
		defer response.Body.Close()

		body, err = io.ReadAll(response.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
			return
		}

		err = json.Unmarshal(body, &groupsResp)
		if err != nil {
			log.Printf("failed to unmarshal response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
			return
		}

	}

	owner_int, err := strconv.Atoi(owner)
	if err != nil {
		log.Printf("failed to atoi owner: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad owner value"})
		return
	}
	group_int, err := strconv.Atoi(group)
	if err != nil {
		log.Printf("failed to atoi group: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad group value"})
		return
	}

	c.HTML(200, "edit-form.html", gin.H{
		"admin":        admin,
		"volume":       volume,
		"resourcename": resourcename,
		"rid":          rid,
		"owner":        owner_int,
		"group":        group_int,
		"perms":        parsePermissionsString(perms),
		"users":        usersResp.Content,
		"groups":       groupsResp.Content,
	})
}

func (srv *HTTPService) handleHasher(c *gin.Context) {
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var hashereq struct {
		HashAlg  string `json:"hashalg" form:"hashalg"`
		HashText string `json:"hash" form:"hash"`
		Text     string `json:"text" form:"text"`
		HashCost int    `json:"hashcost" form:"hashcost"`
	}

	if err := c.ShouldBind(&hashereq); err != nil {
		log.Printf("binding error: %v", err)
		// Respond with the appropriate error on the template.
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad binding"})
		return
	}

	if hashereq.Text == "" && hashereq.HashText == "" {
		c.JSON(404, gin.H{"error": "empty  request.."})
		return
	}

	jsonData, err := json.Marshal(hashereq)
	if err != nil {
		log.Printf("error marshalling request data: %v", err)
		c.JSON(500, gin.H{"error": "failed to marshal"})
		return
	}
	req, err := http.NewRequest(http.MethodPost, authServiceURL+authVersion+"/admin/hasher", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch users"})
		return
	}
	defer response.Body.Close()

	var resp struct {
		Result string `json:"result"`
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	err = json.Unmarshal(body, &resp)
	if err != nil {
		log.Printf("failed to unmarshal response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	// Render the HTML template
	c.String(http.StatusOK, "%v", resp.Result)
}

func (srv *HTTPService) handleDashboard(c *gin.Context) {
	uid, _ := c.Get("user_id")
	username, _ := c.Get("username")
	// gids, _ := c.Get("group_ids")
	// group_names, _ := c.Get("groups")
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// we need to fetch bunch of data here
	// 0) fetch user info
	uReq, err := http.NewRequest(http.MethodGet, authServiceURL+authVersion+"/admin/users?uid="+fmt.Sprintf("%v", uid), nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	uReq.Header.Set("Authorization", "Bearer "+accessToken)
	uReq.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))
	// 1) fetch user volumes
	// uvReq, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/admin/user/volume?uids="+fmt.Sprintf("%v", uid), nil)
	// if err != nil {
	// 	log.Printf("failed to create request: %v", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	// 	return
	// }
	// uvReq.Header.Set("Authorization", "Bearer "+accessToken)
	// uvReq.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

	// // 2) fetch group volumes
	// gvReq, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/admin/group/volume?gids="+fmt.Sprintf("%v", gids), nil)
	// if err != nil {
	// 	log.Printf("failed to create request: %v", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	// 	return
	// }
	// gvReq.Header.Set("Authorization", "Bearer "+accessToken)
	// gvReq.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

	// // 3) fetch user jobs
	// jReq, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/job?uids="+fmt.Sprintf("%v", uid), nil)
	// if err != nil {
	// 	log.Printf("failed to create request: %v", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	// 	return
	// }
	// jReq.Header.Set("Authorization", "Bearer "+accessToken)
	// jReq.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

	// // 4) fetch user resources

	// rReq, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/resources?struct=content", nil)
	// if err != nil {
	// 	log.Printf("failed to create request: %v", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	// 	return
	// }
	// rReq.Header.Set("Authorization", "Bearer "+accessToken)
	// rReq.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))
	// rReq.Header.Set("Access-Target", fmt.Sprintf("/ %v:%v", uid, gids))

	// client := &http.Client{Timeout: 10 * time.Second}
	// var (
	// 	uResp struct {
	// 		Content []ut.User `json:"content"`
	// 	}
	// 	uvResp struct {
	// 		Content []ut.UserVolume `json:"content"`
	// 	}
	// 	gvResp struct {
	// 		Content []ut.GroupVolume `json:"content"`
	// 	}
	// 	jResp struct {
	// 		Content []ut.Job `json:"content"`
	// 	}
	// 	rResp struct {
	// 		Content []ut.Resource `json:"content"`
	// 	}
	// 	vResp struct {
	// 		Content []ut.Volume `json:"content"`
	// 	}
	// 	uErr, uvErr, gvErr, jErr, rErr error
	// )

	// wg := sync.WaitGroup{}
	// wg.Add(5)

	// // do the requests as goroutines
	// go func() {
	// 	defer wg.Done()
	// 	resp, err := client.Do(uReq)
	// 	if err != nil {
	// 		uErr = err
	// 		return
	// 	}

	// 	if resp.StatusCode != http.StatusOK {
	// 		uErr = fmt.Errorf("failed to fetch user ; status: %d", resp.StatusCode)
	// 		return
	// 	}
	// 	if err := json.NewDecoder(resp.Body).Decode(&uResp); err != nil {
	// 		uErr = fmt.Errorf("failed to decode user  response: %v", err)
	// 		return
	// 	}
	// }()

	// go func() {
	// 	defer wg.Done()
	// 	resp, err := client.Do(uvReq)
	// 	if err != nil {
	// 		uvErr = err
	// 		return
	// 	}
	// 	defer resp.Body.Close()

	// 	if resp.StatusCode != http.StatusOK {
	// 		uvErr = fmt.Errorf("failed to fetch user volumes; status: %d", resp.StatusCode)
	// 		return
	// 	}
	// 	if err := json.NewDecoder(resp.Body).Decode(&uvResp); err != nil {
	// 		uvErr = fmt.Errorf("failed to decode user volume response: %v", err)
	// 		return
	// 	}
	// }()

	// go func() {
	// 	defer wg.Done()
	// 	resp, err := client.Do(gvReq)
	// 	if err != nil {
	// 		gvErr = err
	// 		return
	// 	}
	// 	defer resp.Body.Close()

	// 	if resp.StatusCode != http.StatusOK {
	// 		gvErr = fmt.Errorf("failed to fetch group volumes; status: %d", resp.StatusCode)
	// 		return
	// 	}
	// 	if err := json.NewDecoder(resp.Body).Decode(&gvResp); err != nil {
	// 		gvErr = fmt.Errorf("failed to decode group volume response: %v", err)
	// 		return
	// 	}
	// }()

	// go func() {
	// 	defer wg.Done()
	// 	resp, err := client.Do(jReq)
	// 	if err != nil {
	// 		jErr = err
	// 		return
	// 	}
	// 	defer resp.Body.Close()

	// 	if resp.StatusCode != http.StatusOK {
	// 		jErr = fmt.Errorf("failed to fetch jobs; status: %d", resp.StatusCode)
	// 		return
	// 	}
	// 	if err := json.NewDecoder(resp.Body).Decode(&jResp); err != nil {
	// 		jErr = fmt.Errorf("failed to decode jobs response: %v", err)
	// 		return
	// 	}
	// }()

	// go func() {
	// 	defer wg.Done()
	// 	resp, err := client.Do(rReq)
	// 	if err != nil {
	// 		rErr = err
	// 		return
	// 	}
	// 	defer resp.Body.Close()

	// 	if resp.StatusCode != http.StatusOK {
	// 		rErr = fmt.Errorf("failed to fetch resources; status: %d", resp.StatusCode)
	// 		return
	// 	}
	// 	if err := json.NewDecoder(resp.Body).Decode(&rResp); err != nil {
	// 		rErr = fmt.Errorf("failed to decode resources response: %v", err)
	// 		return
	// 	}
	// }()
	// wg.Wait()

	// if uErr != nil || uvErr != nil || gvErr != nil || jErr != nil || rErr != nil {
	// 	log.Printf("failed to fetch data: %v %v, %v, %v, %v", uErr, uvErr, gvErr, jErr, rErr)
	// 	c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch data"})
	// 	return
	// }

	// // we need to fetch also the volumes we are eating from,
	// var vids_map map[int]bool = make(map[int]bool)
	// for _, uv := range uvResp.Content {
	// 	vids_map[uv.Vid] = true
	// }
	// for _, gv := range gvResp.Content {
	// 	vids_map[gv.Vid] = true
	// }

	// vids := make([]int, 0, len(vids_map))
	// for vid, is := range vids_map {
	// 	if is {
	// 		vids = append(vids, vid)
	// 	}
	// }
	// vReq, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/admin/volumes?vids="+strings.Trim(strings.Join(strings.Fields(fmt.Sprint(vids)), ","), "[]"), nil)
	// if err != nil {
	// 	log.Printf("failed to create request: %v", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	// 	return
	// }

	// vReq.Header.Set("Authorization", "Bearer "+accessToken)
	// vReq.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))
	// resp, err := client.Do(vReq)
	// if err != nil {
	// 	rErr = err
	// 	return
	// }
	// defer resp.Body.Close()

	// if resp.StatusCode != http.StatusOK {
	// 	rErr = fmt.Errorf("failed to fetch volumes; status: %d", resp.StatusCode)
	// 	return
	// }
	// if err := json.NewDecoder(resp.Body).Decode(&vResp); err != nil {
	// 	rErr = fmt.Errorf("failed to decode volumes response: %v", err)
	// 	return
	// }

	// log.Printf("User Volumes Response: %+v", uvResp.Content)
	// log.Printf("Group Volumes Response: %+v", gvResp.Content)
	// // log.Printf("Jobs Response: %+v", jResp.Content)
	// // log.Printf("Resources Response: %+v", rResp.Content)
	// // log.Printf("Volumes Response: %+v", vResp.Content)

	// Render the HTML template
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"message":  "Welcome to your dashboard, ",
		"username": username,
		// 	"groups":          group_names,
		// 	"info":            uResp.Content[0].Info,
		// 	"home":            uResp.Content[0].Home,
		// 	"jobs":            jResp.Content,
		// 	"total_jobs":      len(jResp.Content),
		// 	"resources":       rResp.Content,
		// 	"total_resources": len(rResp.Content),
		// 	"user_volume":     uvResp.Content[0],
		// 	"groups_volume":   gvResp.Content,
	})

}

func (srv *HTTPService) passwordChangeHandler(c *gin.Context) {
	// whoami?
	username, exists := c.Get("username")
	if !exists {
		log.Printf("username doesn't exist, can't continue")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "couldn't authenticate user"})
		return
	}
	access_token, exists := c.Get("access_token")
	if !exists {
		log.Printf("access_token was not set, unauthorized")
		c.JSON(http.StatusForbidden, gin.H{"error": "failed to retrieve access_token"})
		return
	}

	var cpass struct {
		CurPass    string `json:"current_password" form:"current_password"`
		NewPass    string `json:"new_password" form:"new_password"`
		NewPassRep string `json:"new_password_repeat" form:"new_password_repeat"`
	}

	// parse req
	err := c.ShouldBind(&cpass)
	if err != nil {
		log.Printf("failed to bind request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}

	// verify password
	// must match
	if cpass.NewPass != cpass.NewPassRep {
		log.Printf("passwords don't match")
		c.JSON(http.StatusBadRequest, gin.H{"error": "passwords don't match"})
		return
	}
	// check if password  is correct
	data, err := json.Marshal(gin.H{"username": username, "password": cpass.CurPass})
	if err != nil {
		log.Printf("failed to marshal data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal data"})
		return
	}
	checkPassReq, err := http.NewRequest(http.MethodPost, authServiceURL+authVersion+"/admin/verify-password", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("failed to create a new requst: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	checkPassReq.Header.Add("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))

	resp, err := http.DefaultClient.Do(checkPassReq)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("invalid")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid"})
		return
	}

	// forward password change
	data, err = json.Marshal(gin.H{"username": username, "password": cpass.NewPass})
	if err != nil {
		log.Printf("failed to marshal data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal data"})
		return
	}
	passReq, err := http.NewRequest(http.MethodPost, authServiceURL+authVersion+"/passwd", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("failed to create a new requst: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	// passReq.Header.Add("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))
	passReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", access_token))

	resp2, err := http.DefaultClient.Do(passReq)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	defer resp2.Body.Close()
	c.Status(resp2.StatusCode)
	for key, values := range resp2.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	_, err = io.Copy(c.Writer, resp2.Body)
	if err != nil {
		log.Printf("failed to write response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write response"})
	}
}

func (srv *HTTPService) updateUser(c *gin.Context) {
	// get request body (this request)
	email := c.Request.FormValue("new-email-change")
	if email == "" {
		log.Printf("empty email value, must speicify")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide an email"})
		return
	}

	// identify urself
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	// get current user info
	req, err := http.NewRequest(http.MethodGet, authServiceURL+authVersion+"/user/me", nil)
	if err != nil {
		log.Printf("failed to create a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	// req.Header.Add("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to perform the request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "internal server error"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read the response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	log.Printf("body returned: %+v", string(body))

	var users []ut.User
	err = json.Unmarshal(body, &users)
	if err != nil && len(users) != 1 {
		log.Printf("error, failed to unmarshal the body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// apply change
	// log.Printf("user updated: %+v", user)

	var userFormat struct {
		Uid  int    `json:"uid"`
		Info string `json:"info"`
	}
	userFormat.Uid = users[0].Uid
	userFormat.Info = email

	user_data, err := json.Marshal(userFormat)
	if err != nil {
		log.Printf("failed to marshal user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	// save change (forward to update)
	newReq, err := http.NewRequest(http.MethodPatch, authServiceURL+authVersion+"/admin/userpatch", bytes.NewBuffer(user_data))
	if err != nil {
		log.Printf("failed to create a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	newReq.Header.Set("X-Service-Secret", string(srv.Config.SERVICE_SECRET_KEY))
	newReq.Header.Set("Content-Type", "application/json")

	resp2, err := http.DefaultClient.Do(newReq)
	if err != nil {
		log.Printf("failed to perform the request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	defer resp2.Body.Close()

	// respond
	c.Status(resp2.StatusCode)
	for key, values := range resp2.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	_, err = io.Copy(c.Writer, resp2.Body)
	if err != nil {
		log.Printf("failed to write response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write response"})
	}
}

func (srv *HTTPService) handleAdminPanel(c *gin.Context) {
	username := c.GetString("username")
	groups := c.GetString("groups")
	info := c.GetString("info")
	home := c.GetString("home")
	elevated := 0
	if strings.Contains(groups, "admin") {
		elevated = 1
	}

	c.HTML(http.StatusOK, "admin-panel.html", gin.H{
		"username": username,
		"message":  "Welcome to the Admin Panel ",
		"info":     info,
		"home":     home,
		"elevated": elevated,
		"groups":   groups,
	})

}

/*
*********************************************************************
* */
/* helpful functions */
func jsonPostRequest(destinationURI string, accessToken string, requestData any) (*http.Response, error) {
	// Marshal the request data into JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("error marshalling request data: %w", err)
	}

	// Create a new POST request with the JSON data
	req, err := http.NewRequest(http.MethodPost, destinationURI, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	// Use an HTTP client to send the request
	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

func parseTreeNode(name string, data map[string]any) *TreeNode {
	if isFileNode(data) {
		var resource ut.Resource
		jsonData, _ := json.Marshal(data)
		_ = json.Unmarshal(jsonData, &resource)

		return &TreeNode{
			Name:     name,
			Type:     "file",
			Resource: &resource,
		}
	}

	node := &TreeNode{
		Name:     name,
		Type:     "directory",
		Children: make(map[string]*TreeNode),
	}

	for key, value := range data {
		if childData, ok := value.(map[string]any); ok {
			node.Children[key] = parseTreeNode(key, childData)
		}
	}

	return node
}

func isFileNode(data map[string]any) bool {
	_, hasName := data["name"]
	_, hasType := data["type"]
	return hasName && hasType
}

func respondInFormat(c *gin.Context, format string, data any, template_name string) {
	switch format {
	case "json":
		c.JSON(http.StatusOK, data)
	default:
		c.HTML(http.StatusOK, template_name, data)
	}
}

// parsePermissionsString translates something like "rwxr-xr--" into a FilePermissions struct.
func parsePermissionsString(permsStr string) FilePermissions {
	// We assume permsStr has length >= 9 (like "rwxr-xr--").
	fp := FilePermissions{}
	if len(permsStr) < 9 {
		return fp // or handle error; for safety
	}
	fp.OwnerR = permsStr[0] == 'r'
	fp.OwnerW = permsStr[1] == 'w'
	fp.OwnerX = permsStr[2] == 'x'

	fp.GroupR = permsStr[3] == 'r'
	fp.GroupW = permsStr[4] == 'w'
	fp.GroupX = permsStr[5] == 'x'

	fp.OtherR = permsStr[6] == 'r'
	fp.OtherW = permsStr[7] == 'w'
	fp.OtherX = permsStr[8] == 'x'

	return fp
}

// buildPermissionsString goes the other way around (if you need to reconstruct the string):
func BuildPermissionsString(fp FilePermissions) string {
	// Convert booleans back into 'r', 'w', 'x' or '-'
	return string([]rune{
		boolChar(fp.OwnerR, 'r'),
		boolChar(fp.OwnerW, 'w'),
		boolChar(fp.OwnerX, 'x'),
		boolChar(fp.GroupR, 'r'),
		boolChar(fp.GroupW, 'w'),
		boolChar(fp.GroupX, 'x'),
		boolChar(fp.OtherR, 'r'),
		boolChar(fp.OtherW, 'w'),
		boolChar(fp.OtherX, 'x'),
	})
}

func boolChar(b bool, c rune) rune {
	if b {
		return c
	}
	return '-'
}

func compareStatus(status1, status2 string) bool {
	if status1 == "completed" || status1 == "pending" && (status2 == "pending" || status2 == "failed") || (status1 == "failed" && status2 == "") {
		return true
	} else {
		return false
	}
}

/* some structs that are used in the requests */
type LoginRequest struct {
	Username string `form:"username" json:"username" binding:"required,min=3,max=20"`
	Password string `form:"password" json:"password" binding:"required,min=4,max=100"`
}
type RegisterRequest struct {
	Username       string `form:"username" json:"username" binding:"required,min=3,max=20"`
	Password       string `form:"password" json:"password" binding:"required,min=4,max=100"`
	RepeatPassword string `form:"repeat-password" json:"repeat-password" binding:"required,min=4,max=100"`
	Email          string `form:"email" json:"email"`
}

type AuthServiceResponse struct {
	Token   string `json:"token"`
	Role    string `json:"role"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

type UseraddClaim struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
	Email    string `json:"email" form:"email"`
	Home     string `json:"home" form:"home"`
}

/* This is the same response for useradd and register*/
type RegResponse struct {
	Message   string `json:"message"`
	Login_url string `json:"login_url"`
	Uid       int    `json:"uid"`
	Pgroup    int    `json:"pgroup"`
}

type TreeNode struct {
	Resource *ut.Resource         `json:"resource,omitempty"`
	Children map[string]*TreeNode `json:"children,omitempty"`
	Name     string               `json:"name,omitempty"`
	Type     string               `json:"type"` // "directory" or "file"
}

type FilePermissions struct {
	OwnerR bool
	OwnerW bool
	OwnerX bool
	GroupR bool
	GroupW bool
	GroupX bool
	OtherR bool
	OtherW bool
	OtherX bool
}
