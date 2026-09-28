package frontendapp

import (
	"net/http"
	"strconv"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

/*
	Accounts: login, registration and the user's own settings (minioth),
	plus the file edit form and the admin hasher.
*/

// LoginRequest struct to send a login request to AuthService
type LoginRequest struct {
	Username string `binding:"required,min=3,max=20"  form:"username" json:"username"`
	Password string `binding:"required,min=4,max=100" form:"password" json:"password"`
}

// RegisterRequest struct to send a register request to Authservice
type RegisterRequest struct {
	Username       string `binding:"required,min=3,max=20"  form:"username"       json:"username"`
	Password       string `binding:"required,min=4,max=100" form:"password"       json:"password"`
	RepeatPassword string `binding:"required,min=4,max=100" form:"repeatPassword" json:"repeatPassword"`
	Email          string `form:"email"                     json:"email"`
}

// UseraddClaim is an admin's "add user" form.
type UseraddClaim struct {
	Username string `form:"username" json:"username"`
	Password string `form:"password" json:"password"`
	Email    string `form:"email"    json:"email"`
	Home     string `form:"home"     json:"home"`
}

// RegResponse is minioth's answer to a registration.
type RegResponse struct {
	Message  string `json:"message"`
	LoginURL string `json:"login_url"` //nolint:tagliatelle // minioth wire format (snake_case)
	UID      int64  `json:"uid"`
	Pgroup   int64  `json:"pgroup"`
}

type passChange struct {
	CurPass    string `form:"currentPassword"   json:"currentPassword"`
	NewPass    string `form:"newPassword"       json:"newPassword"`
	NewPassRep string `form:"newPasswordRepeat" json:"newPasswordRepeat"`
}

func (srv *HTTPService) handleLogin(c *gin.Context) {
	var login LoginRequest
	if err := c.ShouldBind(&login); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad binding"})

		return
	}
	// minioth answers with a flat, snake_case shape; only the token is kept
	var resp struct {
		AccessToken string `json:"access_token"` //nolint:tagliatelle // minioth wire format (snake_case)
	}
	err := srv.minioth.decode(c.Request.Context(), request{method: http.MethodPost, path: "/v1/login", json: login}, &resp)
	if err != nil {
		fail(c, "log in", err)

		return
	}
	if _, err := srv.verifier.Verify(resp.AccessToken); err != nil {
		// a token this frontapp can't verify would make every page fail
		c.JSON(http.StatusBadGateway, gin.H{"error": "the identity service issued an unusable token"})

		return
	}
	setSessionCookie(c, sessionCookie, resp.AccessToken, 3600)
	c.Redirect(http.StatusSeeOther, "/api/v1/verified/admin-panel")
}

// handleRegister creates the account (minioth also creates the user's
// group) and gives the user their storage.
func (srv *HTTPService) handleRegister(c *gin.Context) {
	var reg RegisterRequest
	if err := c.ShouldBind(&reg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Register binding"})

		return
	}
	if reg.Password != reg.RepeatPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Passwords should match"})

		return
	}
	var created RegResponse
	err := srv.minioth.decode(c.Request.Context(), request{
		method: http.MethodPost, path: "/v1/register",
		json: gin.H{"user": ut.User{
			Username: reg.Username, Info: reg.Email, Home: "/home/" + reg.Username, Shell: "gshell",
			Password: ut.Password{Hashpass: reg.Password},
		}},
	}, &created)
	if err != nil {
		fail(c, "register", err)

		return
	}
	if err := srv.claimStorage(c.Request.Context(), created.UID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "registered, but failed to set up user storage"})

		return
	}
	c.Redirect(http.StatusSeeOther, "/api/v1/login")
}

// passwordChangeHandler changes the session user's password after checking
// the current one.
func (srv *HTTPService) passwordChangeHandler(c *gin.Context) {
	var cp passChange
	if err := c.ShouldBind(&cp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})

		return
	}
	if cp.NewPass != cp.NewPassRep {
		c.JSON(http.StatusBadRequest, gin.H{"error": "passwords don't match"})

		return
	}
	username := c.GetString("username")
	ctx := c.Request.Context()
	if err := srv.minioth.decode(ctx, request{
		method: http.MethodPost, path: "/v1/admin/verify-password", svc: true,
		json: gin.H{"username": username, "password": cp.CurPass},
	}, nil); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current password not matched"})

		return
	}
	resp, err := srv.minioth.do(ctx, request{ //nolint:bodyclose // relay closes it
		method: http.MethodPost, path: "/v1/passwd", token: userToken(c),
		json: gin.H{"username": username, "password": cp.NewPass},
	})
	if err != nil {
		fail(c, "change password", err)

		return
	}
	relay(c, resp)
}

// updateUser changes the session user's email. The user id comes from the
// verified session (it was looked up with an extra /user/me call).
func (srv *HTTPService) updateUser(c *gin.Context) {
	email := c.PostForm("new-email-change")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide an email"})

		return
	}
	uid, err := strconv.ParseInt(c.GetString("userID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bad session"})

		return
	}
	resp, err := srv.minioth.do(c.Request.Context(), request{ //nolint:bodyclose // relay closes it
		method: http.MethodPatch, path: "/v1/admin/userpatch", svc: true,
		json: gin.H{"uid": uid, "info": email},
	})
	if err != nil {
		fail(c, "update email", err)

		return
	}
	relay(c, resp)
}

// editFormHandler renders a file's edit form; admins also get the user and
// group lists to change owner and group.
func (srv *HTTPService) editFormHandler(c *gin.Context) {
	name, rid := c.Query("resourcename"), c.Query("rid")
	owner, err1 := strconv.Atoi(c.Query("owner"))
	group, err2 := strconv.Atoi(c.Query("group"))
	perms := c.Query("perms")
	if name == "" || rid == "" || perms == "" || err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide resourcename, rid, owner, group and perms"})

		return
	}
	var users struct {
		Content []ut.User `json:"content"`
	}
	var groups struct {
		Content []ut.Group `json:"content"`
	}
	admin := isAdmin(c)
	if admin {
		ctx := c.Request.Context()
		if err := srv.minioth.decode(ctx, request{method: http.MethodGet, path: "/v1/admin/users", token: userToken(c)}, &users); err != nil {
			fail(c, "list users", err)

			return
		}
		if err := srv.minioth.decode(ctx, request{method: http.MethodGet, path: "/v1/admin/groups", token: userToken(c)}, &groups); err != nil {
			fail(c, "list groups", err)

			return
		}
		for i := range users.Content {
			users.Content[i].Password.Hashpass = ""
		}
	}
	p, err := parsePermissionsString(perms)
	if err != nil { // default rw-r--r--
		p.Owner.Read, p.Owner.Write, p.Group.Read, p.Other.Read = true, true, true, true
	}
	c.HTML(http.StatusOK, "edit-form.html", gin.H{
		"admin": admin, "volume": c.Query("volume"), "resourcename": name, "rid": rid,
		"owner": owner, "group": group, "perms": p,
		"users": users.Content, "groups": groups.Content,
	})
}

func (srv *HTTPService) handleHasher(c *gin.Context) {
	var req struct {
		HashAlg  string `form:"hashalg"  json:"hashalg"`
		HashText string `form:"hash"     json:"hash"`
		Text     string `form:"text"     json:"text"`
		HashCost int    `form:"hashcost" json:"hashcost"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad binding"})

		return
	}
	if req.Text == "" && req.HashText == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty request"})

		return
	}
	var resp struct {
		Result string `json:"result"`
	}
	if err := srv.minioth.decode(c.Request.Context(), request{
		method: http.MethodPost, path: "/v1/admin/hasher", token: userToken(c), json: req,
	}, &resp); err != nil {
		fail(c, "hash", err)

		return
	}
	c.String(http.StatusOK, "%v", resp.Result)
}
