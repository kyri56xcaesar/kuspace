package frontendapp

import (
	"context"
	"net/http"
	"net/url"
	"sort"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

/*
	User and group administration: minioth's admin API, called with the
	admin's own token (minioth checks the admin group).
*/

func (srv *HTTPService) handleFetchUsers(c *gin.Context) {
	var resp struct {
		Content []ut.User `json:"content"`
	}
	err := srv.minioth.decode(c.Request.Context(), request{method: http.MethodGet, path: "/v1/admin/users", token: userToken(c)}, &resp)
	if err != nil {
		fail(c, "list users", err)

		return
	}
	// minioth includes the bcrypt hash in /admin/users; never forward it to the browser
	for i := range resp.Content {
		resp.Content[i].Password.Hashpass = ""
	}
	sort.Slice(resp.Content, func(i, j int) bool { return resp.Content[i].UID < resp.Content[j].UID })
	respondInFormat(c, c.Query("format"), resp.Content, "users_template.html")
}

// handleUseradd creates a user (like registration, without its checks) and
// gives them storage.
func (srv *HTTPService) handleUseradd(c *gin.Context) {
	var uac UseraddClaim
	if err := c.ShouldBind(&uac); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Register binding"})

		return
	}
	var created RegResponse
	err := srv.minioth.decode(c.Request.Context(), request{
		method: http.MethodPost, path: "/v1/admin/useradd", token: userToken(c),
		json: gin.H{"user": ut.User{
			Username: uac.Username, Info: uac.Email, Home: "/home/" + uac.Username, Shell: "gshell",
			Password: ut.Password{Hashpass: uac.Password},
		}},
	}, &created)
	if err != nil {
		fail(c, "add user", err)

		return
	}
	// users added by an admin used to get no storage until uspace restarted
	if err := srv.claimStorage(c.Request.Context(), created.UID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "user added, but setting up their storage failed"})

		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "user added"})
}

// claimStorage gives a new user their share of the default volume. It is
// a service call: the user has no session yet (or it's an admin acting).
func (srv *HTTPService) claimStorage(ctx context.Context, uid int64) error {
	return srv.uspace.decode(ctx, request{
		method: http.MethodPost, path: "/api/v1/admin/user/volume", svc: true, target: "0::/ 0:0",
		json: ut.UserVolume{VID: 1, UID: uid}, // uspace maps an unknown vid to the default volume
	}, nil)
}

// minothMessage relays a minioth answer of the form {"message": ...} as text.
func (srv *HTTPService) minothMessage(c *gin.Context, what string, r request) {
	r.token = userToken(c)
	var resp struct {
		Message string `json:"message"`
	}
	if err := srv.minioth.decode(c.Request.Context(), r, &resp); err != nil {
		fail(c, what, err)

		return
	}
	c.String(http.StatusOK, "%v", resp.Message)
}

func (srv *HTTPService) handleUserdel(c *gin.Context) {
	uid := c.Query("uid")
	if uid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing uid param"})

		return
	}
	srv.minothMessage(c, "delete user", request{method: http.MethodDelete, path: "/v1/admin/userdel", query: url.Values{"uid": {uid}}})
}

func (srv *HTTPService) handleUserpatch(c *gin.Context) {
	var rq struct {
		Username string `form:"username" json:"username"`
		Password string `form:"password" json:"password"`
		Email    string `form:"info"     json:"info"`
		Home     string `form:"home"     json:"home"`
		Shell    string `form:"shell"    json:"shell"`
		GID      string `form:"pgroup"   json:"pgroup"`
		Groups   string `form:"groups"   json:"groups"`
		UID      int    `form:"uid"      json:"uid"`
	}
	if err := c.ShouldBind(&rq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad binding"})

		return
	}
	srv.minothMessage(c, "update user", request{method: http.MethodPatch, path: "/v1/admin/userpatch", json: rq})
}

func (srv *HTTPService) handleFetchGroups(c *gin.Context) {
	var resp struct {
		Content []ut.Group `json:"content"`
	}
	err := srv.minioth.decode(c.Request.Context(), request{method: http.MethodGet, path: "/v1/admin/groups", token: userToken(c)}, &resp)
	if err != nil {
		fail(c, "list groups", err)

		return
	}
	sort.Slice(resp.Content, func(i, j int) bool { return resp.Content[i].GID < resp.Content[j].GID })
	respondInFormat(c, c.Query("format"), resp.Content, "groups_template.html")
}

func (srv *HTTPService) handleGroupadd(c *gin.Context) {
	var req struct {
		Groupname string `form:"groupname" json:"groupname"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad binding"})

		return
	}
	if err := srv.minioth.decode(c.Request.Context(), request{
		method: http.MethodPost, path: "/v1/admin/groupadd", token: userToken(c), json: req,
	}, nil); err != nil {
		fail(c, "add group", err)

		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "group added"})
}

func (srv *HTTPService) handleGroupdel(c *gin.Context) {
	gid := c.Query("gid")
	if gid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing gid param"})

		return
	}
	srv.minothMessage(c, "delete group", request{method: http.MethodDelete, path: "/v1/admin/groupdel", query: url.Values{"gid": {gid}}})
}

func (srv *HTTPService) handleGrouppatch(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "renaming groups is not supported yet"})
}
