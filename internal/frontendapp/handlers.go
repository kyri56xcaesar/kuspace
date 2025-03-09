package frontendapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	filepath "path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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
	authVersion    string

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

	req, err := http.NewRequest(http.MethodGet, authServiceURL+"/admin/users", nil)
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
		Content []User `json:"content"`
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
	resp, err := forwardPostRequest(authServiceURL+authVersion+"/admin/useradd", accessToken, gin.H{
		"user": User{
			Username: uac.Username,
			Info:     uac.Email,
			Home:     "/home/" + uac.Username,
			Shell:    "gshell",
			Password: Password{
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

	/* give the user some of the volume pie */
	go func() {
		json_data, err := json.Marshal(UserVolume{Vid: 1, Uid: useraddResp.Uid})
		if err != nil {
			log.Printf("failed to marshal to json: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set uv"})
			return
		}
		req, err := http.NewRequest(http.MethodPost, apiServiceURL+"/api/v1/admin/user/volume", bytes.NewBuffer(json_data))
		if err != nil {
			log.Printf("failed to create a new request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set uv"})
			req.Header.Add("X-Service-Secret", string(srv.Config.ServiceSecret))
			return
		}
		req.Header.Add("X-Service-Secret", string(srv.Config.ServiceSecret))

		_, err = http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("failed to send user volume claim request: %v", err)
		}

	}()

	/* give the users primary group some of the pie as well*/
	go func() {
		json_data, err := json.Marshal(GroupVolume{Vid: 1, Gid: useraddResp.Pgroup})
		if err != nil {
			log.Printf("failed to marshal to json: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set gv"})
			return
		}
		req, err := http.NewRequest(http.MethodPost, apiServiceURL+"/api/v1/admin/group/volume", bytes.NewBuffer(json_data))
		if err != nil {
			log.Printf("failed to create a new request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set gv"})
			return
		}
		req.Header.Add("X-Service-Secret", string(srv.Config.ServiceSecret))

		_, err = http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("failed to send user group volume claim request: %v", err)
			return
		}
	}()

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
	req, err := http.NewRequest(http.MethodDelete, authServiceURL+"/admin/userdel?uid="+uid, nil)
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

	req, err := http.NewRequest(http.MethodPatch, authServiceURL+"/admin/userpatch", bytes.NewBuffer(jsonRq))
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

	req, err := http.NewRequest(http.MethodGet, authServiceURL+"/admin/groups", nil)
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
		Content []Group `json:"content"`
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
	resp, err := forwardPostRequest(authServiceURL+authVersion+"/admin/groupadd", accessToken, req)
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
	req, err := http.NewRequest(http.MethodDelete, authServiceURL+"/admin/groupdel?gid="+gid, nil)
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

	struc_type := c.DefaultQuery("struct", "list")

	req, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/resources?struct="+struc_type, nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	req.Header.Set("Access-Target", "/ 0:0")
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))
	// req.Header.Set("Authorization", "Bearer "+acc)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Printf("failed to make request: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch users"})
		return
	}
	defer response.Body.Close()

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
		var data map[string]interface{}
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
		var data []Resource
		err = json.Unmarshal(body, &data)
		if err != nil {
			log.Printf("failed to unmarshal response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
			return
		}
		c.HTML(http.StatusOK, "list-resources.html", data)
	}
}

func (srv *HTTPService) handleResourceUpload(c *gin.Context) {
	uid, _ := c.Get("user_id")
	group_ids, _ := c.Get("group_ids")

	req, err := http.NewRequest(http.MethodPost, apiServiceURL+"/api/v1/resource/upload", c.Request.Body)
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
	req.Header.Add("Access-Target", fmt.Sprintf("/ %v:%v", uid, group_ids))
	req.Header.Add("Authorization", c.Request.Header.Get("Authorization"))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))

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
	req.Header.Add("Access-Target", fmt.Sprintf("%v %v:%v", fpath, uid, group_ids))
	req.Header.Add("Authorization", c.Request.Header.Get("Authorization"))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))

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
	req.Header.Add("Access-Target", fmt.Sprintf("%v %v:%v", r_name, whoami, groups))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))

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
	access_token, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("failed to retrieve access token cookie: %v", err)
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
		return
	}

	rid := c.Request.URL.Query().Get("rid")
	r_name := c.Request.URL.Query().Get("resourcename")
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
	// form value:

	req, err := http.NewRequest(http.MethodPatch, apiServiceURL+"/api/v1/resource/mv?rid="+rid, c.Request.Body)
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

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+access_token)
	req.Header.Add("Access-Target", fmt.Sprintf("%v %v:%v", r_name, user_id, gids))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))
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
	resource_target := c.Request.URL.Query().Get("rids")
	if resource_target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing target parameter"})
		return
	}
	uid, _ := c.Get("user_id")
	group_ids, _ := c.Get("group_ids")

	req, err := http.NewRequest(http.MethodDelete, apiServiceURL+"/api/v1/resource/rm?rids="+resource_target, c.Request.Body)
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

	req.Header.Add("Access-Target", fmt.Sprintf("/ %v:%v", uid, group_ids))
	req.Header.Add("Authorization", c.Request.Header.Get("Authorization"))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))

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

	req.Header.Add("Authorization", "Bearer "+access_token)
	req.Header.Add("Access-Target", fmt.Sprintf("%v %v:%v", r_name, user_id, gids))
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))
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

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Access-Target", fmt.Sprintf("/ %v:%v", whoami, mygroups))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))

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
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userReq, err := http.NewRequest(http.MethodGet, authServiceURL+"/admin/users", nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	userReq.Header.Set("Authorization", "Bearer "+accessToken)
	userReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))

	groupReq, err := http.NewRequest(http.MethodGet, authServiceURL+"/admin/groups", nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	groupReq.Header.Set("Authorization", "Bearer "+accessToken)
	groupReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))

	volumeReq, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/admin/volumes", nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	volumeReq.Header.Set("Authorization", "Bearer "+accessToken)
	volumeReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))

	uvReq, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/admin/user/volume?uids=*", nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	uvReq.Header.Set("Authorization", "Bearer "+accessToken)
	uvReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))

	gvReq, err := http.NewRequest(http.MethodGet, apiServiceURL+"/api/v1/admin/group/volume?gids=*", nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	gvReq.Header.Set("Authorization", "Bearer "+accessToken)
	gvReq.Header.Set("X-Service-Secret", string(srv.Config.ServiceSecret))

	client := &http.Client{Timeout: 10 * time.Second}
	var (
		userResp struct {
			Content []User `json:"content"`
		}
		groupResp struct {
			Content []Group `json:"content"`
		}
		volumeResp struct {
			Content []Volume `json:"content"`
		}
		userVolumesResp struct {
			Content []UserVolume `json:"content"`
		}
		groupVolumesResp struct {
			Content []GroupVolume `json:"content"`
		}
	)

	var userErr, groupErr, volumeErr, uvError, gvError error

	var wg sync.WaitGroup
	wg.Add(5)

	// 1) fetch Users
	go func() {
		defer wg.Done() // signals that this goroutine is finished
		resp, err := client.Do(userReq)
		if err != nil {
			userErr = err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			userErr = fmt.Errorf("failed to fetch users; status: %d", resp.StatusCode)
			return
		}

		if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
			userErr = fmt.Errorf("failed to decode users response: %v", err)
			return
		}
	}()

	// 2) fetch Groups
	go func() {
		defer wg.Done()
		resp, err := client.Do(groupReq)
		if err != nil {
			groupErr = err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			groupErr = fmt.Errorf("failed to fetch groups; status: %d", resp.StatusCode)
			return
		}

		if err := json.NewDecoder(resp.Body).Decode(&groupResp); err != nil {
			groupErr = fmt.Errorf("failed to decode groups response: %v", err)
			return
		}
	}()

	// 3) fetch Volumes
	go func() {
		defer wg.Done()
		resp, err := client.Do(volumeReq)
		if err != nil {
			volumeErr = err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			volumeErr = fmt.Errorf("failed to fetch volumes; status: %d", resp.StatusCode)
			return
		}
		if err := json.NewDecoder(resp.Body).Decode(&volumeResp); err != nil {
			volumeErr = fmt.Errorf("failed to decode volume response: %v", err)
			return
		}
	}()

	// 4) fetch userVolumes
	go func() {
		defer wg.Done()
		resp, err := client.Do(uvReq)
		if err != nil {
			uvError = err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			uvError = fmt.Errorf("failed to fetch user volumes; status: %d", resp.StatusCode)
			return
		}
		if err := json.NewDecoder(resp.Body).Decode(&userVolumesResp); err != nil {
			uvError = fmt.Errorf("failed to decode user volume response: %v", err)
			return
		}
	}()

	// 5) fetch groupVolumes
	go func() {
		defer wg.Done()
		resp, err := client.Do(gvReq)
		if err != nil {
			gvError = err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			gvError = fmt.Errorf("failed to fetch group volumes; status: %d", resp.StatusCode)
			return
		}
		if err := json.NewDecoder(resp.Body).Decode(&groupVolumesResp); err != nil {
			gvError = fmt.Errorf("failed to decode group volume response: %v", err)
			return
		}
	}()

	wg.Wait()

	if userErr != nil {
		log.Printf("user request error: %v", userErr)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch users"})
		return
	}
	if groupErr != nil {
		log.Printf("group request error: %v", groupErr)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch groups"})
		return
	}

	if volumeErr != nil {
		log.Printf("volume request error: %v", volumeErr)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch volumes"})
		return
	}

	if uvError != nil {
		log.Printf("user volume request error: %v", uvError)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch user volumes"})
		return
	}

	if gvError != nil {
		log.Printf("group volume request error: %v", gvError)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch group volumes"})
		return
	}

	sort.Slice(userResp.Content, func(i, j int) bool {
		return userResp.Content[i].Uid < userResp.Content[j].Uid
	})

	sort.Slice(groupResp.Content, func(i, j int) bool {
		return groupResp.Content[i].Gid < groupResp.Content[j].Gid
	})

	combinedData := gin.H{
		"users":         userResp.Content,
		"groups":        groupResp.Content,
		"volumes":       volumeResp.Content,
		"user_volumes":  userVolumesResp.Content,
		"group_volumes": groupVolumesResp.Content,
	}

	format := c.Request.URL.Query().Get("format")

	// Render the HTML template
	respondInFormat(c, format, combinedData, "volumes_template.html")
}

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
	resp, err := forwardPostRequest(authServiceURL+authVersion+"/login", "", login)
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
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Username     string `json:"username"`
		Groups       string `json:"groups"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
		log.Printf("Error decoding auth service response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serious"})
		return
	}

	c.SetCookie("access_token", authResponse.AccessToken, 3600, "/api/v1/", "", false, true)
	c.SetCookie("refresh_token", authResponse.RefreshToken, 3600, "/api/v1/", "", false, true)

	if strings.Contains(authResponse.Groups, "admin") {
		c.Redirect(http.StatusSeeOther, "/api/v1/verified/admin-panel")
	} else {
		c.Redirect(http.StatusSeeOther, "/api/v1/verified/dashboard")
	}
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

	data := User{
		Username: reg.Username,
		Info:     reg.Email,
		Home:     "/home/" + reg.Username,
		Shell:    "gshell",
		Password: Password{
			Hashpass: reg.Password,
		},
	}
	// Forward login request to the auth service
	resp, err := forwardPostRequest(authServiceURL+authVersion+"/register", "", gin.H{
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
		json_data, err := json.Marshal(GroupVolume{Vid: 1, Gid: authResponse.Pgroup})
		if err != nil {
			log.Printf("failed to marshal to json: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set uv"})
			return
		}
		req, err := http.NewRequest(http.MethodPost, apiServiceURL+"/api/v1/admin/group/volume", bytes.NewBuffer(json_data))
		if err != nil {
			log.Printf("failed to create a new request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set gv"})
			req.Header.Add("X-Service-Secret", string(srv.Config.ServiceSecret))
			return
		}
		req.Header.Add("X-Service-Secret", string(srv.Config.ServiceSecret))

		_, err = http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("failed to send user group volume claim request: %v", err)
			return
		}

	}()
	/* let user some of the volume pie*/
	go func() {
		json_data, err := json.Marshal(UserVolume{Vid: 1, Uid: authResponse.Uid})
		if err != nil {
			log.Printf("failed to marshal to json: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set uv"})
			return
		}
		req, err := http.NewRequest(http.MethodPost, apiServiceURL+"/api/v1/admin/user/volume", bytes.NewBuffer(json_data))
		if err != nil {
			log.Printf("failed to create a new request: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set uv"})
			req.Header.Add("X-Service-Secret", string(srv.Config.ServiceSecret))
			return
		}
		req.Header.Add("X-Service-Secret", string(srv.Config.ServiceSecret))

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
	resourcename := c.Request.URL.Query().Get("resourcename")
	rid := c.Request.URL.Query().Get("rid")
	owner := c.Request.URL.Query().Get("owner")
	group := c.Request.URL.Query().Get("group")
	perms := c.Request.URL.Query().Get("perms")

	if resourcename == "" || owner == "" || group == "" || perms == "" || rid == "" {
		log.Printf("must provide args")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide information"})
		return
	}

	// need to request for all the users and all the groups... again..
	// should implement a caching mechanism asap...
	accessToken, err := c.Cookie("access_token")
	if err != nil {
		log.Printf("missing access_token cookie: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	usersReq, err := http.NewRequest(http.MethodGet, authServiceURL+"/admin/users", nil)
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

	var usersResp struct {
		Content []User `json:"content"`
	}

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

	groupsReq, err := http.NewRequest(http.MethodGet, authServiceURL+"/admin/groups", nil)
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

	var groupsResp struct {
		Content []Group `json:"content"`
	}

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
	req, err := http.NewRequest(http.MethodPost, authServiceURL+"/admin/hasher", bytes.NewBuffer(jsonData))
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

func (srv *HTTPService) addSymlinkFormHandler(c *gin.Context) {
	c.HTML(200, "add-symlink-forhtml", nil)
}

/*
*********************************************************************
* */
/* helpful functions */
func forwardPostRequest(destinationURI string, accessToken string, requestData interface{}) (*http.Response, error) {
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

func parseTreeNode(name string, data map[string]interface{}) *TreeNode {
	if isFileNode(data) {
		var resource Resource
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
		if childData, ok := value.(map[string]interface{}); ok {
			node.Children[key] = parseTreeNode(key, childData)
		}
	}

	return node
}

func isFileNode(data map[string]interface{}) bool {
	_, hasName := data["name"]
	_, hasType := data["type"]
	return hasName && hasType
}

func respondInFormat(c *gin.Context, format string, data interface{}, template_name string) {
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
