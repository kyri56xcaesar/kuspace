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
	authVersion    = "/v1"

	apiServiceURL string
	wssServiceURL string
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
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})

		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authServiceURL+authVersion+"/admin/users", nil)
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
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close body: %v", err)
		}
	}()

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
		return resp.Content[i].UID < resp.Content[j].UID
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
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
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
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close body: %v", err)
		}
	}()
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

/*
	Forward a user deletion request to the AUTH service
	*
	* perform some authoriaztion checks meanwhile...
	*
	*

return if deletion succeeded or not.
*/
func (srv *HTTPService) handleUserdel(c *gin.Context) {
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})

		return
	}

	uid := c.Request.URL.Query().Get("uid")
	if uid == "" {
		log.Printf("missing uid parameter, must provide...")
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing uid param"})

		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, authServiceURL+authVersion+"/admin/userdel?uid="+uid, nil)
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
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close body: %v", err)
		}
	}()

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
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})

		return
	}

	var rq struct {
		Username string `form:"username" json:"username"`
		Password string `form:"password" json:"password"`
		Email    string `form:"info" json:"info"`
		Home     string `form:"home" json:"home"`
		Shell    string `form:"shell" json:"shell"`
		GID      string `form:"pgroup" json:"pgroup"`
		Groups   string `form:"groups" json:"groups"`
		UID      int    `form:"uid" json:"uid"`
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, authServiceURL+authVersion+"/admin/userpatch", bytes.NewBuffer(jsonRq))
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
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close body: %v", err)
		}
	}()

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
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})

		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authServiceURL+authVersion+"/admin/groups", nil)
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
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

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
		return resp.Content[i].GID < resp.Content[j].GID
	})

	// Render the HTML template

	format := c.Request.URL.Query().Get("format")

	// Render the HTML template
	respondInFormat(c, format, resp.Content, "groups_template.html")
}

func (srv *HTTPService) handleGroupadd(c *gin.Context) {
	/* Since, this is an admin function, verify early that access token exists
	* perhaps, unnecessary.
	* */
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})

		return
	}

	var req struct {
		Groupname string `form:"groupname" json:"groupname"`
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
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close body: %v", err)
		}
	}()

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
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})

		return
	}

	gid := c.Request.URL.Query().Get("gid")
	if gid == "" {
		log.Printf("missing gid parameter, must provide...")
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing gid param"})

		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, authServiceURL+authVersion+"/admin/groupdel?gid="+gid, nil)
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
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

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

func (srv *HTTPService) handleGrouppatch(_ *gin.Context) {
}

/*
*********************************************************************
*   Resources
* */
func (srv *HTTPService) handleFetchResources(c *gin.Context) {
	_, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})

		return
	}

	volume := c.Query("volume")
	if volume == "" {
		volume = "*"
	}
	structType := c.DefaultQuery("struct", "list")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiServiceURL+"/api/v1/resources?struct="+structType, nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

		return
	}
	req.Header.Set("Access-Target", fmt.Sprintf(":%s:/ 0:0", volume))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
	// req.Header.Set("Authorization", "Bearer "+acc)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch users"})

		return
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

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

	switch structType {
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
	uid, _ := c.Get("userID")
	groupIDs, _ := c.Get("groupIDs")
	volume := c.Query("volume")
	if volume == "" {
		volume = c.Request.Header.Get("X-Volume-Target")
		if volume == "" {
			volume = srv.Config.MinioDefaultBucket
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

	req.Header.Set("Access-Target", fmt.Sprintf("0:%s:/ %v:%v", volume, uid, groupIDs))
	req.Header.Set("Authorization", c.Request.Header.Get("Authorization"))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to upload resource"})

		return
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close body: %v", err)
		}
	}()

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
	uid, _ := c.Get("userID")
	groupIDs, _ := c.Get("groupIDs")
	fpath := c.Request.URL.Query().Get("target")

	if fpath == "" {
		log.Printf("must provide a target")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a target"})

		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiServiceURL+"/api/v1/resource/download", c.Request.Body)
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
		volume = srv.Config.MinioDefaultBucket
	}

	req.Header.Add("Access-Target", fmt.Sprintf(":%s:%v %v:%v", volume, fpath, uid, groupIDs))
	req.Header.Add("Authorization", c.Request.Header.Get("Authorization"))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))

	log.Printf("%v %v:%v", fpath, uid, groupIDs)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to upload resource"})

		return
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()
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
	whoami, exists := c.Get("userID")
	groups, gexists := c.Get("groupIDs")
	if !exists || !gexists {
		log.Printf("uid or gids don't exist... bad authentication")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bad auth"})

		return
	}
	rid := c.Request.URL.Query().Get("rid")
	rName := c.Request.URL.Query().Get("resourcename")
	if rName == "" || rid == "" {
		log.Printf("must provide resource name")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide resource name"})

		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiServiceURL+"/api/v1/resource/preview?rid="+rid, c.Request.Body)
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
		volume = srv.Config.MinioDefaultBucket
	}

	req.Header.Add("Access-Target", fmt.Sprintf(":%s:%v %v:%v", volume, rName, whoami, groups))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("request error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed request"})

		return
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()
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
	rName := c.Query("resourcename")
	if rName == "" {
		log.Printf("must provide resource name")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide resource name"})

		return
	}

	volume := c.Query("volume")
	if volume == "" {
		volume = srv.Config.MinioDefaultBucket
	}

	newName := c.PostForm("resourcename")
	if err := ut.ValidateObjectName(newName); err != nil { // should check for name validity as well!
		log.Printf("invalid name: %s: %v", newName, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "should provide a proper new name"})

		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, apiServiceURL+"/api/v1/resource/mv?dest="+volume+"/"+newName, nil)
	if err != nil {
		log.Printf("error creating a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failure"})

		return
	}

	userID, exists := c.Get("userID")
	gids, gexists := c.Get("groupIDs")
	if !exists || !gexists {
		log.Printf("uid or gids don't exist... bad authentication")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bad auth"})

		return
	}

	req.Header.Set("Access-Target", fmt.Sprintf(":%s:%v %v:%v", volume, rName, userID, gids))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to upload resource"})

		return
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()
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
	resourceTarget := c.Request.URL.Query().Get("name")
	if resourceTarget == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing target parameter"})

		return
	}
	uid, _ := c.Get("userID")
	groupIDs, _ := c.Get("groupIDs")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiServiceURL+"/api/v1/resource/rm", c.Request.Body)
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
		volume = srv.Config.MinioDefaultBucket
	}

	req.Header.Add("Access-Target", fmt.Sprintf(":%s:%s %v:%v", volume, resourceTarget, uid, groupIDs))
	req.Header.Add("Authorization", c.Request.Header.Get("Authorization"))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to upload resource"})

		return
	}

	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

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
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("failed to retrieve access token cookie: %v", err)
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})

		return
	}

	rid := c.Request.URL.Query().Get("rid")

	rName := c.Request.URL.Query().Get("resource")
	if rName == "" || rid == "" {
		log.Printf("must provide resource name")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide resource name"})

		return
	}

	// should filter input
	// if err := utils.allowedFileName(filename); err != nil {
	//log.Printf("bad resource name: %v", err)
	//c.JSON(http.StatusBadRequest, gin.H{"error":"bad resource name"})
	// return
	//}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, apiServiceURL+"/api/v1/resource/cp?rid="+rid, c.Request.Body)
	if err != nil {
		log.Printf("error creating a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failure"})

		return
	}

	userID, exists := c.Get("userID")
	gids, gexists := c.Get("groupIDs")
	if !exists || !gexists {
		log.Printf("uid or gids don't exist... bad authentication")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bad auth"})

		return
	}
	volume := c.Query("volume")
	if volume == "" {
		volume = srv.Config.MinioDefaultBucket
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Access-Target", fmt.Sprintf(":%s:%v %v:%v", volume, rName, userID, gids))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to upload resource"})

		return
	}

	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

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
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err = http.NewRequestWithContext(ctx, http.MethodPatch, apiServiceURL+endpoint, requestBody)
	if err != nil {
		log.Printf("failed to create a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create a new request"})

		return
	}

	whoami, exists := c.Get("userID")
	mygroupIDs, gexists := c.Get("groupIDs")
	if !exists || !gexists {
		log.Printf("uid or groups were not set correctly. Authencitation fail")
		c.JSON(http.StatusInsufficientStorage, gin.H{"error": "failed auth"})

		return
	}
	volume := c.Query("volume")
	if volume == "" {
		volume = srv.Config.MinioDefaultBucket
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Access-Target", fmt.Sprintf(":%s:$rids=%s %v:%v", volume, rid, whoami, mygroupIDs))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to upload resource"})

		return
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	volumeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiServiceURL+"/api/v1/admin/volumes", nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

		return
	}
	volumeReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
	volumeReq.Header.Set("Access-Target", "0::/ 0:0")

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
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	if err := json.NewDecoder(response.Body).Decode(&volumeResp); err != nil {
		log.Printf("volume request error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch volumes"})

		return
	}
	format := c.Request.URL.Query().Get("format")

	groups := c.GetString("groups")
	elevated := 0
	if strings.Contains(groups, "admin") {
		elevated = 1
	}

	combinedData := gin.H{
		"volumes":  volumeResp.Content,
		"elevated": elevated,
	}
	// Render the HTML template
	respondInFormat(c, format, combinedData, "volumes_template.html")
}

func (srv *HTTPService) handleVolumeadd(c *gin.Context) {
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})

		return
	}
	var req ut.Volume
	if err := c.ShouldBind(&req); err != nil {
		log.Printf("failed to bind request: %v", err)
		c.JSON(400, gin.H{"error": "bad binding"})

		return
	}

	//  should sanitize the volume input

	/* forward login request to the auth service */
	jsonData, err := json.Marshal(req)
	if err != nil {
		log.Printf("failed to marshal data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to bind json data"})

		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	// Create a new POST request with the JSON data
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiServiceURL+"/api/v1/admin/volumes", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("failed to create a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create a new request"})

		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
	request.Header.Set("Access-Target", "0::/ 0:0")

	// Use an HTTP client to send the request
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Do(request)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch jobs"})

		return
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

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
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})

		return
	}

	vname := c.Query("volume")
	if vname == "" {
		log.Printf("missing vname parameter, must provide...")
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing vname param"})

		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiServiceURL+"/api/v1/admin/volumes?volume="+vname, nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})

		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
	req.Header.Set("Access-Target", "0::/ 0:0")

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to delete user"})

		return
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})

		return
	}
	c.Data(response.StatusCode, "application/json", body)
}

/*
*********************************************************************
*   Jobs
* */
func (srv *HTTPService) jobsHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	switch c.Request.Method {
	case http.MethodGet:
		jobReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiServiceURL+"/api/v1/job", nil)
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

			return
		}
		jobReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
		jobReq.Header.Set("Access-Target", "0::/ 0:0")

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(jobReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch jobs"})

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()

		var jobResp struct {
			Content []ut.Job `json:"content"`
			Admin   int      `json:"admin"`
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
				return jobResp.Content[i].UID > jobResp.Content[j].UID
			})
		case "jid":
			sort.Slice(jobResp.Content, func(i, j int) bool {
				return jobResp.Content[i].JID > jobResp.Content[j].JID
			})
		case "status":
			sort.Slice(jobResp.Content, func(i, j int) bool {
				return compareStatus(jobResp.Content[i].Status, jobResp.Content[j].Status)
			})
		case "createdAt", "time":
			// sort users on time
			sort.Slice(jobResp.Content, func(i, j int) bool {
				t1, err1 := time.Parse(ut.TimeFormat, jobResp.Content[i].CreatedAt)
				t2, err2 := time.Parse(ut.TimeFormat, jobResp.Content[j].CreatedAt)

				// Handle parsing errors gracefully (e.g., keep original order)
				if err1 != nil || err2 != nil {
					return false
				}

				return t1.After(t2)
			})
		default:
			// sort users on time
			sort.Slice(jobResp.Content, func(i, j int) bool {
				t1, err1 := time.Parse(ut.TimeFormat, jobResp.Content[i].CreatedAt)
				t2, err2 := time.Parse(ut.TimeFormat, jobResp.Content[j].CreatedAt)

				// Handle parsing errors gracefully (e.g., keep original order)
				if err1 != nil || err2 != nil {
					return false
				}

				return t1.After(t2)
			})
		}

		// answer according to format
		format := c.Request.URL.Query().Get("format")

		groups := c.GetString("groups")
		elevated := 0
		if strings.Contains(groups, "admin") {
			elevated = 1
		}
		jobResp.Admin = elevated

		// Render the HTML template
		respondInFormat(c, format, jobResp, "jobs_list_template.html")
	case http.MethodPost:
		// lets fix the uid (identify ourselves)
		var job ut.Job
		err := c.ShouldBind(&job)
		if err != nil {
			log.Printf("failed to bind json body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind"})

			return
		}

		err = job.ValidateForm(srv.Config.UspaceJobMaxCPU, srv.Config.UspaceJobMaxMemory, srv.Config.UspaceJobMaxStorage,
			int64(srv.Config.UspaceJobMaxParallelism), srv.Config.UspaceJobMaxTimeout, srv.Config.UspaceJobMaxLogicSize)
		if err != nil {
			log.Printf("failed to validate form: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

			return
		}

		// our uid
		uid, exists := c.Get("userID")
		if !exists {
			log.Printf("uid not set correctly... should be unreachable")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "incosiderable"})

			return
		}
		job.UID, err = strconv.Atoi(uid.(string))
		if err != nil {
			log.Printf("failed to atoi uid value: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to atoi uid"})

			return
		}
		jobJSON, err := json.Marshal(job)
		if err != nil {
			log.Printf("failed to marshal job: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal job"})

			return
		}
		jobReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiServiceURL+"/api/v1/job", bytes.NewBuffer(jobJSON))
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

			return
		}
		jobReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(jobReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch jobs"})

			return
		}

		var resp struct {
			JID    int    `json:"jid"`
			Status string `json:"status"`
		}

		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
		err = json.Unmarshal(body, &resp)
		if err != nil {
			log.Printf("failed to unmarshal response body: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to retrieve expected response"})

			return
		}

		c.JSON(http.StatusOK, gin.H{"jid": resp.JID, "status": resp.Status, "output": job.Output})
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not supported"})
	}
}

func (srv *HTTPService) jobAdminHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	switch c.Request.Method {
	case http.MethodGet:
		jobReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiServiceURL+"/api/v1/job", nil)
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

			return
		}
		jobReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
		jobReq.Header.Set("Access-Target", "0::/ 0:0")

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(jobReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch jobs"})

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
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
				return jobResp.Content[i].UID > jobResp.Content[j].UID
			})
		case "jid":
			sort.Slice(jobResp.Content, func(i, j int) bool {
				return jobResp.Content[i].JID > jobResp.Content[j].JID
			})
		case "status":
			sort.Slice(jobResp.Content, func(i, j int) bool {
				return compareStatus(jobResp.Content[i].Status, jobResp.Content[j].Status)
			})
		case "createdAt", "time":
			// sort users on time
			sort.Slice(jobResp.Content, func(i, j int) bool {
				t1, err1 := time.Parse(ut.TimeFormat, jobResp.Content[i].CreatedAt)
				t2, err2 := time.Parse(ut.TimeFormat, jobResp.Content[j].CreatedAt)

				// Handle parsing errors gracefully (e.g., keep original order)
				if err1 != nil || err2 != nil {
					return false
				}

				return t1.After(t2)
			})
		default:
			// sort users on time
			sort.Slice(jobResp.Content, func(i, j int) bool {
				t1, err1 := time.Parse(ut.TimeFormat, jobResp.Content[i].CreatedAt)
				t2, err2 := time.Parse(ut.TimeFormat, jobResp.Content[j].CreatedAt)

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

		err = job.ValidateForm(srv.Config.UspaceJobMaxCPU, srv.Config.UspaceJobMaxMemory, srv.Config.UspaceJobMaxStorage,
			int64(srv.Config.UspaceJobMaxParallelism), srv.Config.UspaceJobMaxTimeout, srv.Config.UspaceJobMaxLogicSize)
		if err != nil {
			log.Printf("failed to validate form: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

			return
		}

		// our uid
		uid, exists := c.Get("userID")
		if !exists {
			log.Printf("uid not set correctly... should be unreachable")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "incosiderable"})

			return
		}
		job.UID, err = strconv.Atoi(uid.(string))
		if err != nil {
			log.Printf("failed to atoi uid value: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to atoi uid"})

			return
		}
		jobJSON, err := json.Marshal(job)
		if err != nil {
			log.Printf("failed to marshal job: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal job"})

			return
		}
		jobReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiServiceURL+"/api/v1/admin/job", bytes.NewBuffer(jobJSON))
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

			return
		}
		jobReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
		jobReq.Header.Set("Access-Target", "0::/ 0:0")

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(jobReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch jobs"})

			return
		}

		var resp struct {
			JID    int    `json:"jid"`
			Status string `json:"status"`
		}

		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
		err = json.Unmarshal(body, &resp)
		if err != nil {
			log.Printf("failed to unmarshal response body: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to retrieve expected response"})

			return
		}

		c.JSON(http.StatusOK, gin.H{"jid": resp.JID, "status": resp.Status, "output": job.Output})
	case http.MethodPut:
		var job ut.Job
		err := c.ShouldBind(&job)
		if err != nil {
			log.Printf("failed to bind json body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind"})

			return
		}

		err = job.ValidateForm(srv.Config.UspaceJobMaxCPU, srv.Config.UspaceJobMaxMemory, srv.Config.UspaceJobMaxStorage,
			int64(srv.Config.UspaceJobMaxParallelism), srv.Config.UspaceJobMaxTimeout, srv.Config.UspaceJobMaxLogicSize)
		if err != nil {
			log.Printf("failed to validate form: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

			return
		}

		jobJSON, err := json.Marshal(job)
		if err != nil {
			log.Printf("failed to marshal job: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal job"})

			return
		}
		jobReq, err := http.NewRequestWithContext(ctx, http.MethodPut, apiServiceURL+"/api/v1/admin/job", bytes.NewBuffer(jobJSON))
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

			return
		}
		jobReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
		jobReq.Header.Set("Access-Target", "0::/ 0:0")
		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(jobReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch jobs"})

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})

			return
		}
		c.Data(response.StatusCode, "application/json", body)
	case http.MethodDelete:
		jid := c.Query("jid")
		if jid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a jid"})

			return
		}
		jobReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiServiceURL+"/api/v1/admin/job?jid="+jid, nil)
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

			return
		}
		jobReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
		jobReq.Header.Set("Access-Target", "0::/ 0:0")

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(jobReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to delete job"})

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})

			return
		}
		c.Data(response.StatusCode, "application/json", body)
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not supported"})
	}
}

func (srv *HTTPService) appsHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	switch c.Request.Method {
	case http.MethodGet:
		appReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiServiceURL+"/api/v1/app", nil)
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

			return
		}
		appReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(appReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch data"})

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
		var resp struct {
			Content []ut.Application `json:"content"`
			Admin   int              `json:"admin"`
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
				return resp.Content[i].AuthorID > resp.Content[j].AuthorID
			})
		case "createdAt", "time":
			// sort users on time
			sort.Slice(resp.Content, func(i, j int) bool {
				t1, err1 := time.Parse(ut.TimeFormat, resp.Content[i].CreatedAt)
				t2, err2 := time.Parse(ut.TimeFormat, resp.Content[j].CreatedAt)

				// Handle parsing errors gracefully (e.g., keep original order)
				if err1 != nil || err2 != nil {
					return false
				}

				return t1.After(t2)
			})
		case "insertedAt":
			// sort users on time
			sort.Slice(resp.Content, func(i, j int) bool {
				t1, err1 := time.Parse(ut.TimeFormat, resp.Content[i].InsertedAt)
				t2, err2 := time.Parse(ut.TimeFormat, resp.Content[j].InsertedAt)

				// Handle parsing errors gracefully (e.g., keep original order)
				if err1 != nil || err2 != nil {
					return false
				}

				return t1.After(t2)
			})
		}

		// answer according to format
		format := c.Request.URL.Query().Get("format")

		groups := c.GetString("groups")
		elevated := 0
		if strings.Contains(groups, "admin") {
			elevated = 1
		}
		resp.Admin = elevated

		// Render the HTML template
		respondInFormat(c, format, resp, "apps_list_template.html")

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not supported"})
	}
}

func (srv *HTTPService) appAdminHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	switch c.Request.Method {
	case http.MethodGet:
		appReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiServiceURL+"/api/v1/app", nil)
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

			return
		}
		appReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(appReq)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch data"})

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
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
				return resp.Content[i].AuthorID > resp.Content[j].AuthorID
			})
		case "createdAt", "time":
			// sort users on time
			sort.Slice(resp.Content, func(i, j int) bool {
				t1, err1 := time.Parse(ut.TimeFormat, resp.Content[i].CreatedAt)
				t2, err2 := time.Parse(ut.TimeFormat, resp.Content[j].CreatedAt)

				// Handle parsing errors gracefully (e.g., keep original order)
				if err1 != nil || err2 != nil {
					return false
				}

				return t1.After(t2)
			})
		case "insertedAt":
			// sort users on time
			sort.Slice(resp.Content, func(i, j int) bool {
				t1, err1 := time.Parse(ut.TimeFormat, resp.Content[i].InsertedAt)
				t2, err2 := time.Parse(ut.TimeFormat, resp.Content[j].InsertedAt)

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
		var app ut.Application
		err := c.ShouldBind(&app)
		if err != nil {
			log.Printf("failed to bind json body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind"})

			return
		}
		appJSON, err := json.Marshal(app)
		if err != nil {
			log.Printf("failed to marshal app: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal app"})

			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiServiceURL+"/api/v1/admin/app", bytes.NewBuffer(appJSON))
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

			return
		}
		req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
		req.Header.Set("Access-Target", "0::/ 0:0")

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(req)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to post app"})

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
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
	case http.MethodPut:
		var app ut.Application
		err := c.ShouldBind(&app)
		if err != nil {
			log.Printf("failed to bind json body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind"})

			return
		}

		appJSON, err := json.Marshal(app)
		if err != nil {
			log.Printf("failed to marshal app: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal app"})

			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiServiceURL+"/api/v1/admin/app", bytes.NewBuffer(appJSON))
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

			return
		}
		req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
		req.Header.Set("Access-Target", "0::/ 0:0")

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(req)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to put app"})

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
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
	case http.MethodDelete:
		id := c.Query("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a id"})

			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiServiceURL+"/api/v1/admin/app?id="+id, nil)
		if err != nil {
			log.Printf("failed to create request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})

			return
		}
		req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
		req.Header.Set("Access-Target", "0::/ 0:0")

		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Do(req)
		if err != nil {
			log.Printf("failed to make request: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to delete app"})

			return
		}
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})

			return
		}
		c.Data(response.StatusCode, "application/json", body)
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
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

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
		AccessToken string  `json:"accessToken"`
		User        ut.User `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
		log.Printf("Error decoding auth service response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serious"})

		return
	}

	c.SetCookie("accessToken", authResponse.AccessToken, 3600, "/api/v1/", "", false, true)

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
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	/* give user's primary group some of the pie..*/
	go func() {
		jsonData, err := json.Marshal(ut.GroupVolume{VID: 1, GID: authResponse.Pgroup})
		if err != nil {
			log.Printf("failed to marshal to json: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set uv"})

			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiServiceURL+"/api/v1/admin/group/volume", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("failed to create a new request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set gv"})
			req.Header.Add("X-Service-Secret", string(srv.Config.ServiceSecretKey))

			return
		}
		req.Header.Add("X-Service-Secret", string(srv.Config.ServiceSecretKey))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("failed to send user group volume claim request: %v", err)

			return
		}
		err = resp.Body.Close()
		if err != nil {
			log.Printf("failed to close the response body: %v", err)
		}
	}()
	/* let user some of the volume pie*/
	go func() {
		jsonData, err := json.Marshal(ut.UserVolume{VID: 1, UID: authResponse.UID})
		if err != nil {
			log.Printf("failed to marshal to json: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set uv"})

			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiServiceURL+"/api/v1/admin/user/volume", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("failed to create a new request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set uv"})
			req.Header.Add("X-Service-Secret", string(srv.Config.ServiceSecretKey))

			return
		}
		req.Header.Add("X-Service-Secret", string(srv.Config.ServiceSecretKey))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("failed to send user volume claim request: %v", err)

			return
		}
		err = resp.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()
	// at this point registration should be successful, we can directly login the user
	// .. somehow

	// perform login idk bout this
	// nvm
	c.Redirect(http.StatusSeeOther, "/api/v1/login")
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
		accessToken, err := c.Cookie("accessToken")
		if err != nil {
			log.Printf("missing accessToken cookie: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})

			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
		defer cancel()
		usersReq, err := http.NewRequestWithContext(ctx, http.MethodGet, authServiceURL+authVersion+"/admin/users", nil)
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
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
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

		groupsReq, err := http.NewRequestWithContext(ctx, http.MethodGet, authServiceURL+authVersion+"/admin/groups", nil)
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
		defer func() {
			err := response.Body.Close()
			if err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
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

	ownerInt, err := strconv.Atoi(owner)
	if err != nil {
		log.Printf("failed to atoi owner: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad owner value"})

		return
	}
	groupInt, err := strconv.Atoi(group)
	if err != nil {
		log.Printf("failed to atoi group: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad group value"})

		return
	}

	p, err := parsePermissionsString(perms)
	if err != nil {
		// set default perms
		p.Owner.Read = true
		p.Owner.Write = true
		p.Group.Read = true
		p.Other.Read = true
	}
	c.HTML(200, "edit-form.html", gin.H{
		"admin":        admin,
		"volume":       volume,
		"resourcename": resourcename,
		"rid":          rid,
		"owner":        ownerInt,
		"group":        groupInt,
		"perms":        p,
		"users":        usersResp.Content,
		"groups":       groupsResp.Content,
	})
}

func (srv *HTTPService) handleHasher(c *gin.Context) {
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})

		return
	}

	var hashereq struct {
		HashAlg  string `form:"hashalg" json:"hashalg"`
		HashText string `form:"hash" json:"hash"`
		Text     string `form:"text" json:"text"`
		HashCost int    `form:"hashcost" json:"hashcost"`
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authServiceURL+authVersion+"/admin/hasher", bytes.NewBuffer(jsonData))
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
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()
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

func (srv *HTTPService) passwordChangeHandler(c *gin.Context) {
	// whoami?
	username, exists := c.Get("username")
	if !exists {
		log.Printf("username doesn't exist, can't continue")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "couldn't authenticate user"})

		return
	}
	accessToken, exists := c.Get("accessToken")
	if !exists {
		log.Printf("accessToken was not set, unauthorized")
		c.JSON(http.StatusForbidden, gin.H{"error": "failed to retrieve accessToken"})

		return
	}
	var cpass passChange
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	// check if password  is correct
	data, err := json.Marshal(gin.H{"username": username, "password": cpass.CurPass})
	if err != nil {
		log.Printf("failed to marshal data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal data"})

		return
	}
	checkPassReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		authServiceURL+authVersion+"/admin/verify-password", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("failed to create a new requst: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})

		return
	}
	checkPassReq.Header.Add("X-Service-Secret", string(srv.Config.ServiceSecretKey))

	resp, err := http.DefaultClient.Do(checkPassReq)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})

		return
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode >= 400 {
		log.Printf("invalid password")
		c.JSON(http.StatusBadRequest, gin.H{"error": "current password not matched"})

		return
	}

	// forward password change
	data, err = json.Marshal(gin.H{"username": username, "password": cpass.NewPass})
	if err != nil {
		log.Printf("failed to marshal data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal data"})

		return
	}
	passReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		authServiceURL+authVersion+"/passwd", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("failed to create a new requst: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})

		return
	}
	passReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp2, err := http.DefaultClient.Do(passReq)
	if err != nil {
		log.Printf("failed to forward request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})

		return
	}

	defer func() {
		err := resp2.Body.Close()
		if err != nil {
			log.Printf("failed to close body: %v", err)
		}
	}()
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
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Printf("missing accessToken cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()

	// get current user info
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authServiceURL+authVersion+"/user/me", nil)
	if err != nil {
		log.Printf("failed to create a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})

		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to perform the request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "internal server error"})

		return
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read the response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})

		return
	}

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
		UID  int    `json:"uid"`
		Info string `json:"info"`
	}
	userFormat.UID = users[0].UID
	userFormat.Info = email

	userData, err := json.Marshal(userFormat)
	if err != nil {
		log.Printf("failed to marshal user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})

		return
	}

	// save change (forward to update)
	newReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, authServiceURL+authVersion+"/admin/userpatch", bytes.NewBuffer(userData))
	if err != nil {
		log.Printf("failed to create a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})

		return
	}
	newReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
	newReq.Header.Set("Content-Type", "application/json")

	resp2, err := http.DefaultClient.Do(newReq)
	if err != nil {
		log.Printf("failed to perform the request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})

		return
	}
	defer func() {
		err := resp2.Body.Close()
		if err != nil {
			log.Printf("failed to close body: %v", err)
		}
	}()

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
	accessToken, ok := c.Get("accessToken")
	if !ok || accessToken == "" {
		log.Printf("bad state, no accessToken found in context, should have been set by middleware")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "shouldn't be here"})

		return
	}
	username := c.GetString("username")
	groups := c.GetString("groups")
	elevated := 0
	if strings.Contains(groups, "admin") {
		elevated = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authServiceURL+authVersion+"/user/me", nil)
	if err != nil {
		log.Printf("failed to create a new req: %v", err)
		c.JSON(http.StatusInternalServerError, nil)
		c.Abort()

		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken.(string))
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to validate access"})
		c.Abort()

		return
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to read resp body",
		})
		c.Abort()

		return
	}
	var resp []ut.User
	err = json.Unmarshal(body, &resp)
	if err != nil {
		log.Printf("failed to unmarshal response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to parse response",
		})
		c.Abort()

		return
	}
	if resp == nil || len(resp) != 1 {
		log.Printf("failed to retrieve user information")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user info, bad state"})

		return
	}

	c.HTML(http.StatusOK, "admin-panel.html", gin.H{
		"username": username,
		"message":  "Welcome to the Admin Panel ",
		"info":     resp[0].Info,
		"home":     resp[0].Home,
		"elevated": elevated,
		"groups":   groups,
	})
}

func (srv *HTTPService) handleSysConf(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	services := c.Query("services")
	if services == "" || services == "*" {
		services = "uspace,wss,frontapp,minioth"
	}
	parts := strings.Split(strings.TrimSpace(services), ",")
	serv := map[string]map[string]string{}
	for _, service := range parts {
		switch service {
		case "uspace":
			// get uspace conf
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiServiceURL+"/api/v1/admin/system-conf", nil)
			if err != nil {
				log.Printf("[API] failed to create a new request: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create a request"})

				return
			}
			req.Header.Set("Access-Target", "0::/ 0:0")
			req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Printf("failed to perform the request: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "internal server error"})

				return
			}
			defer func() {
				err := resp.Body.Close()
				if err != nil {
					log.Printf("failed to close response body: %v", err)
				}
			}()
			var uspacecfg map[string]string
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("[API] failed to read response body: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read response body"})

				return
			}
			err = json.Unmarshal(body, &uspacecfg)
			if err != nil {
				log.Printf("[API] failed to unmarshal response body: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "failed to unmarshal response body"})

				return
			}
			serv["uspace"] = uspacecfg
		case "frontapp":
			// send our conf
			ucfg, err := ut.ReadConfig("configs/"+srv.Config.ConfigPath, false)
			if err != nil {
				log.Printf("[API_sysConf] failed to read frontapp config: %v", err)

				continue
			}
			serv["frontapp"] = ucfg
		case "wss":
			// get wss conf
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, wssServiceURL+"/system-conf", nil)
			if err != nil {
				log.Printf("[API] failed to create a new request: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create a request"})

				return
			}
			req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Printf("failed to perform the request: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "internal server error"})

				return
			}
			defer func() {
				err := resp.Body.Close()
				if err != nil {
					log.Printf("failed to close response body: %v", err)
				}
			}()
			var wsscfg map[string]string
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("[API] failed to read response body: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read response body"})

				return
			}
			err = json.Unmarshal(body, &wsscfg)
			if err != nil {
				log.Printf("[API] failed to unmarshal response body: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "failed to unmarshal response body"})

				return
			}
			serv["wss"] = wsscfg
		case "minioth":
			// get minioth conf
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, authServiceURL+authVersion+"/admin/system-conf", nil)
			if err != nil {
				log.Printf("[API] failed to create a new request: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create a request"})

				return
			}
			accessToken := c.GetString("accessToken")
			if accessToken == "" {
				log.Printf("[API] could not retrieve access token from context, bad state, should not be here")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "bad state"})

				return
			}
			req.Header.Set("Authorization", "Bearer "+accessToken)
			req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Printf("failed to perform the request: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "internal server error"})

				return
			}
			defer func() {
				err := resp.Body.Close()
				if err != nil {
					log.Printf("failed to close response body: %v", err)
				}
			}()
			var miniothcfg map[string]string
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("[API] failed to read response body: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read response body"})

				return
			}
			err = json.Unmarshal(body, &miniothcfg)
			if err != nil {
				log.Printf("[API] failed to unmarshal response body: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "failed to unmarshal response body"})

				return
			}
			serv["minioth"] = miniothcfg
		}
	}
	format := c.Query("format")
	respondInFormat(c, format, serv, "sys_conf_display.html")
}

func (srv *HTTPService) handleSysMetrics(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiServiceURL+"/api/v1/admin/system-metrics", nil)
	if err != nil {
		log.Printf("[API] failed to create a new request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create a forward request"})

		return
	}
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecretKey))
	req.Header.Set("Access-Target", "0::/ 0:0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[API] failed to perform forward request")
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to forward request"})

		return
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[API] failed to read response body: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read response body"})

		return
	}
	var respB map[string]any
	err = json.Unmarshal(body, &respB)
	if err != nil {
		log.Printf("[API] failed to unmarshal response body: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to unmarshal response body"})

		return
	}
	// log.Printf("metrics: %+v", respB)

	format := c.Query("format")
	respondInFormat(c, format, respB, "metrics_display.html")
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	// Create a new POST request with the JSON data
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destinationURI, bytes.NewBuffer(jsonData))
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
		jsonData, err := json.Marshal(data)
		if err != nil {
			log.Printf("failed to marshal data: %v", err)

			return &TreeNode{}
		}
		err = json.Unmarshal(jsonData, &resource)
		if err != nil {
			log.Printf("failed to unmarshal data: %v", err)

			return &TreeNode{}
		}

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

func respondInFormat(c *gin.Context, format string, data any, templateName string) {
	switch format {
	case "json":
		c.JSON(http.StatusOK, data)
	default:
		c.HTML(http.StatusOK, templateName, data)
	}
}

// parsePermissionsString translates something like "rwxr-xr--" into a FilePermissions struct.
func parsePermissionsString(permsStr string) (ut.Permissions, error) {
	// We assume permsStr has length >= 9 (like "rwxr-xr--").
	fp := ut.Permissions{}

	err := fp.FillFromStr(permsStr)
	if err != nil {
		log.Printf("failed to build the object")
	}

	return fp, err
}

func compareStatus(status1, status2 string) bool {
	if status1 == "completed" || status1 == "pending" &&
		(status2 == "pending" || status2 == "failed") || (status1 == "failed" && status2 == "") {
		return true
	}

	return false
}

type passChange struct {
	CurPass    string `form:"currentPassword" json:"currentPassword"`
	NewPass    string `form:"newPassword" json:"newPassword"`
	NewPassRep string `form:"newPasswordRepeat" json:"newPasswordRepeat"`
}

// LoginRequest struct to send a login request to AuthService
type LoginRequest struct {
	Username string `binding:"required,min=3,max=20" form:"username" json:"username"`
	Password string `binding:"required,min=4,max=100" form:"password" json:"password"`
}

// RegisterRequest struct to send a register request to Authservice
type RegisterRequest struct {
	Username       string `binding:"required,min=3,max=20" form:"username" json:"username"`
	Password       string `binding:"required,min=4,max=100" form:"password" json:"password"`
	RepeatPassword string `binding:"required,min=4,max=100" form:"repeatPassword" json:"repeatPassword"`
	Email          string `form:"email" json:"email"`
}

// AuthServiceResponse struct gets binded to the response body of the request to LoginAUth
type AuthServiceResponse struct {
	Token   string `json:"token"`
	Role    string `json:"role"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

// UseraddClaim an incoming request for adding users
type UseraddClaim struct {
	Username string `form:"username" json:"username"`
	Password string `form:"password" json:"password"`
	Email    string `form:"email" json:"email"`
	Home     string `form:"home" json:"home"`
}

// RegResponse struct gets binded by the request to AuthService
type RegResponse struct {
	Message  string `json:"message"`
	LoginURL string `json:"loginUrl"`
	UID      int    `json:"uid"`
	Pgroup   int    `json:"pgroup"`
}

// TreeNode struct describes the "set" of resources in a tree like representation
// using maps
type TreeNode struct {
	Resource *ut.Resource         `json:"resource,omitempty"`
	Children map[string]*TreeNode `json:"children,omitempty"`
	Name     string               `json:"name,omitempty"`
	Type     string               `json:"type"` // "directory" or "file"
}
