package minioth

import (
	"encoding/json"
	"fmt"
	ut "kyri56xcaesar/kuspace/internal/utils"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// @Summary Register a new user
// @Description Registers a new user with provided credentials and returns UID and primary group.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterClaim true "User registration payload"
// @Success 200 {object} map[string]any "registration successful"
// @Failure 400 {object} map[string]string "Invalid input or failed to register"
// @Failure 403 {object} map[string]string "User already exists"
// @Router /register [post]
func (srv *MService) handleRegister(c *gin.Context) {
	var uclaim RegisterClaim
	err := c.BindJSON(&uclaim)
	if err != nil {
		log.Printf("error binding request body to struct: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Verify user credentials
	err = uclaim.validateUser()
	if err != nil {
		log.Printf("failed to validate: %v", err)
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	// Check for uniquness [ NOTE: Now its done internally ]
	// Proceed with Registration
	uid, pgroup, err := srv.Minioth.Useradd(uclaim.User)
	if err != nil {
		log.Print("failed to add user")
		if strings.Contains(strings.ToLower(err.Error()), "alr") {
			c.JSON(403, gin.H{"error": "already exists!"})
		} else {
			c.JSON(400, gin.H{
				"error": "failed to insert the user",
			})
		}
		return
	}

	username, ok := c.Get("username")
	if !ok {
		username = "[debug]admin"
	}
	logAudit("registration", username.(string), fmt.Sprintf("%+v", uclaim), "success", "admin", c.ClientIP())
	c.JSON(200, gin.H{
		"message":   "registration successful.",
		"uid":       uid,
		"pgroup":    pgroup,
		"login_url": "/v1/login",
	})
}

// @Summary Login a user
// @Description Authenticates a user and returns access and refresh tokens.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginClaim true "User login credentials"
// @Success 200 {object} map[string]any "login successful"
// @Failure 400 {object} map[string]string "Validation or auth failure"
// @Failure 404 {object} map[string]string "User not found"
// @Router /login [post]
func (srv *MService) handleLogin(c *gin.Context) {
	var login_claim LoginClaim
	err := c.BindJSON(&login_claim)
	if err != nil {
		log.Printf("error binding request body to struct: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "binding error"})
		return
	}

	// Verify user credentials
	err = login_claim.validateClaim()
	if err != nil {
		log.Printf("failed to validate: %v", err)
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	user, err := srv.Minioth.Authenticate(login_claim.Username, login_claim.Password)
	if err != nil {
		log.Printf("error: %v", err)
		if strings.Contains(err.Error(), "not found") {
			c.JSON(404, gin.H{"error": "user not found"})
		} else {
			c.JSON(400, gin.H{
				"error": "failed to authenticate",
			})
		}
		return
	}

	strGroups := ut.GroupsToString(user.Groups)
	strGids := ut.GidsToString(user.Groups)

	signingAlg := c.GetHeader("X-Auth-Signing-Alg")
	if signingAlg == "" {
		signingAlg = "HS256" // default
	}

	var token string
	switch signingAlg {
	case "RS256":
		token, err = GenerateAccessRS256JWT(strconv.Itoa(user.Uid), login_claim.Username, strGroups, strGids)
	default:
		token, err = GenerateAccessHS256JWT(strconv.Itoa(user.Uid), login_claim.Username, strGroups, strGids)
	}
	if err != nil {
		log.Printf("failed to generate jwt: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	username, ok := c.Get("username")
	if !ok {
		username = "[debug]admin"
	}
	logAudit("login", username.(string), fmt.Sprintf("%+v", login_claim), "success", "admin", c.ClientIP())
	// for now return detailed information so that frontend is accomodated
	// and followup authorization is provided (ids needed)
	// NOTE: use Authorization header for now.
	c.JSON(200, gin.H{
		"user":         user,
		"access_token": token,
	})
}

// @Summary      Refresh access token
// @Description  Generates new access and refresh tokens using a valid refresh token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        refresh_token  body  object{refresh_token=string}  true  "Refresh Token"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /token/refresh [post]
func handleTokenRefresh(c *gin.Context) {
	var requestBody struct {
		Token string `json:"access_token" binding:"required"`
	}

	if err := c.BindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "access_token required",
		})
		return
	}

	token, err := ParseJWT(requestBody.Token)
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid refresh token",
		})
		return
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid claims",
		})
		return
	}

	username, ok := c.Get("username")
	if !ok {
		username = "[debug]admin"
	}
	logAudit("token_regeneration", username.(string), fmt.Sprintf("%+v", requestBody), "success", "admin", c.ClientIP())

	var newAccessToken string
	switch token.Header["alg"].(string) {
	case "HS256":
		newAccessToken, err = GenerateAccessHS256JWT(claims.Subject, claims.Username, claims.Groups, claims.GroupIDS)
		if err != nil {
			log.Printf("error generating new hs access token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "error generating access_token",
			})
			return
		}
	case "RS256":
		newAccessToken, err = GenerateAccessRS256JWT(claims.Subject, claims.Username, claims.Groups, claims.GroupIDS)
		if err != nil {
			log.Printf("error generating new rs access token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "error generating access_token",
			})
			return
		}
	default:
		log.Printf("invalid alg value")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid alg value (parse or token)"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "new access token generated",
		"access_token": newAccessToken,
	})
}

// @Summary      Validate token
// @Description  Checks if a token is valid and returns claims.
// @Tags         auth
// @Produce      json
// @Param        Authorization  header  string  true  "Bearer token"
// @Success      200  {object}  map[string]any{info=map[string]string}
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /user/token [get]
func handleTokenInfo(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		c.Abort()
		return
	}

	if !strings.Contains(authHeader, "Bearer ") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must contain Bearer token"})
		c.Abort()
		return
	}
	// Extract the token from the Authorization header
	tokenString := authHeader[len("Bearer "):]
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token is required"})
		c.Abort()
		return
	}

	// Parse and validate the token
	token, err := ParseJWT(tokenString)

	if err != nil {
		log.Printf("%v token, exiting", token)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "bad token",
		})
		c.Abort()
		return
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		log.Printf("not okay when retrieving claims")
		return
	}

	response := make(map[string]string)
	response["valid"] = strconv.FormatBool(token.Valid)
	response["user_id"] = claims.Subject
	response["username"] = claims.Username
	response["groups"] = claims.Groups
	response["group_ids"] = claims.GroupIDS
	response["issued_at"] = claims.IssuedAt.String()
	response["expires_at"] = claims.ExpiresAt.String()

	c.JSON(http.StatusOK, gin.H{
		"info": response,
	})
}

// @Summary      Get user information
// @Description  Returns full user info based on the access token.
// @Tags         auth
// @Produce      json
// @Param        Authorization  header  string  true  "Bearer token"
// @Success      200  {object}  map[string]any
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /user/me [get]
func (srv *MService) handleTokenUserInfo(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		log.Printf("no auth header")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		c.Abort()
		return
	}

	if !strings.Contains(authHeader, "Bearer ") {
		log.Printf("no bearer")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must contain Bearer token"})
		c.Abort()
		return
	}
	// Extract the token from the Authorization header
	tokenString := authHeader[len("Bearer "):]
	if tokenString == "" {
		log.Printf("no bearer token found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token is required"})
		c.Abort()
		return
	}

	// Parse and validate the token
	token, err := ParseJWT(tokenString)
	if err != nil {
		log.Printf("%v token, exiting", token)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "bad token",
		})
		c.Abort()
		return
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		log.Printf("not okay when retrieving claims")
		return
	}

	user := srv.Minioth.Select("users?uid=" + claims.Subject)

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "not found"})
		return
	} else {
		c.JSON(http.StatusOK, user)
	}
}

/* This endpoint should change a user password. It must "authenticate" the user. User can only change his password. */
// @Summary      Change password
// @Description  Allows a user to change their password.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        credentials  body  object{username=string,password=string}  true  "User Credentials"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /passwd [post]
func (srv *MService) handlePasswd(c *gin.Context) {
	var lclaim LoginClaim
	err := c.BindJSON(&lclaim)
	if err != nil {
		log.Printf("error binding request body to struct: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "binding error"})
		return
	}
	// password change should be used only byourselves or admin, get username by middleware
	username, ok := c.Get("username")
	if !ok {
		log.Printf("failed to authenticate")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "didn't parse bearer token"})
		return
	} else if username != lclaim.Username {
		if groups, _ := c.Get("groups"); !strings.Contains(groups.(string), "admin") {
			log.Printf("not admin trying to change password of another user")
			c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
			return
		}
	}

	pass := ut.Password{
		Hashpass: lclaim.Password,
	}
	// Verify user credentials
	if lclaim.Password == "" {
		c.JSON(400, gin.H{
			"error": "no password provided",
		})
		return
	} else if err := pass.ValidatePassword(); err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	err = srv.Minioth.Passwd(lclaim.Username, lclaim.Password)
	if err != nil {
		log.Printf("failed to change password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to change password"})
		return
	}

	logAudit("user_password_change", username.(string), fmt.Sprintf("%+v", lclaim), "success", "admin", c.ClientIP())
	c.JSON(200, gin.H{"status": "password changed successfully"})
}

// @Summary      Get audit logs
// @Description  Retrieves recent audit logs from the server. Optional query param 'max' specifies the max number of bytes to read.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        max  query     int  false  "Maximum number of bytes to fetch from the audit log"
// @Success      200  {object}  map[string]any  "Audit logs returned successfully"
// @Failure      400  {object}  map[string]string  "Invalid request parameters"
// @Failure      500  {object}  map[string]string  "Failed to read or open audit log"
// @Router       /admin/audit/logs [get]
func handleAuditLogs(c *gin.Context) {
	mx, err := strconv.Atoi(c.Query("max"))
	if err != nil || mx <= 0 {
		mx = audit_log_max_fetch
	}

	lines, err := ut.TailFileLines(audit_log_path, mx)
	if err != nil {
		log.Printf("failed to read audit logs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read audit log"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": lines})
}

// just a login with no token issueing
// @Summary      Verify password
// @Description  Authenticates a user using username and password without issuing a token.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        credentials  body  object{username=string,password=string}  true  "User Credentials"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /admin/verify-password [post]
func (srv *MService) handleVerifyPassword(c *gin.Context) {
	var lclaim LoginClaim
	err := c.BindJSON(&lclaim)
	if err != nil {
		log.Printf("error binding request body to struct: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "binding error"})
		return
	}

	// Verify user credentials
	err = lclaim.validateClaim()
	if err != nil {
		log.Printf("failed to validate: %v", err)
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	_, err = srv.Minioth.Authenticate(lclaim.Username, lclaim.Password)
	if err != nil {
		log.Printf("error: %v", err)
		if strings.Contains(err.Error(), "not found") {
			c.JSON(404, gin.H{"error": "user not found"})
		} else {
			c.JSON(400, gin.H{
				"error": "invalid",
			})
		}
		return
	}
	username, ok := c.Get("username")
	if !ok {
		username = "[debug]admin"
	}
	logAudit("user_authentication", username.(string), fmt.Sprintf("%+v", lclaim), "success", "admin", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"status": "valid"})

}

// @Summary      Hash or verify text
// @Description  Hashes a plaintext input or verifies a hash if both are provided.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        input  body  object{text=string,hash=string,hashcost=int}  true  "Text or Hash Verification Input"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /admin/hasher [post]
func handleHasher(c *gin.Context) {
	var b struct {
		HashAlg  string `json:"hashalg,omitempty"`
		HashText string `json:"hash,omitempty"`
		Text     string `json:"text"`
		HashCost int    `json:"hashcost"`
	}
	err := c.BindJSON(&b)
	if err != nil || (b.HashText == "" && b.Text == "") {
		log.Printf("error binding request body to struct: %v", err)
		c.JSON(400, gin.H{"error": "binding"})
		return
	}

	hashed, err := hash_cost([]byte(b.Text), b.HashCost)
	if err != nil {
		log.Printf("error hasing the text: %v", err)
		c.JSON(500, gin.H{"error": "hashing"})
		return
	}

	if b.HashText == "" {
		c.JSON(200, gin.H{"result": string(hashed)})
	} else {
		c.JSON(200, gin.H{"result": strconv.FormatBool(verifyPass([]byte(b.HashText), []byte(b.Text)))})
	}
}

// @Summary      Get users
// @Description  Retrieves user(s), optionally filtered by UID.
// @Tags         admin
// @Produce      json
// @Param        uid  query  string  false  "User ID"
// @Success      200  {object}  map[string]any{content=[]ut.User}
// @Router       /admin/users [get]
func (srv *MService) handleUsers(c *gin.Context) {
	users := srv.Minioth.Select("users?uid=" + c.Request.URL.Query().Get("uid"))

	c.JSON(http.StatusOK, gin.H{
		"content": users,
	})
}

// @Summary      Get groups
// @Description  Retrieves all groups.
// @Tags         admin
// @Produce      json
// @Success      200  {object}  map[string]any{content=[]ut.Group}
// @Router       /admin/groups [get]
func (srv *MService) handleGroups(c *gin.Context) {
	groups := srv.Minioth.Select("groups")

	c.JSON(http.StatusOK, gin.H{
		"content": groups,
	})
}

/* same as register but dont verify content */
// @Summary Add a new user
// @Tags admin
// @Accept json
// @Produce json
// @Param data body RegisterClaim true "User registration info"
// @Success 200 {object} map[string]any
// @Failure 400,403 {object} map[string]string
// @Router /admin/useradd [post]
func (srv *MService) handleUseradd(c *gin.Context) {
	var uclaim RegisterClaim
	err := c.BindJSON(&uclaim)
	if err != nil {
		log.Printf("error binding request body to struct: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	uid, pgroup, err := srv.Minioth.Useradd(uclaim.User)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "") {
			c.JSON(403, gin.H{"error": "already exists!"})
		} else {
			log.Print("failed to add user")
			c.JSON(400, gin.H{
				"error": "failed to insert the user",
			})
		}
		return
	}

	username, ok := c.Get("username")
	if !ok {
		username = "[debug]admin"
	}
	logAudit("user_add", username.(string), "user: "+uclaim.User.ToString(), "success", "admin", c.ClientIP())
	// TODO: should insta "pseudo" login issue a token for registration.
	// can I redirect to login?
	c.JSON(200, gin.H{
		"message":   fmt.Sprintf("User %v added.", uid),
		"uid":       uid,
		"pgroup":    pgroup,
		"login_url": "sure",
	})
}

// @Summary Delete a user
// @Tags admin
// @Param uid query string true "User ID to delete"
// @Success 200 {object} map[string]string
// @Failure 400,404,500 {object} map[string]string
// @Router /admin/userdel [delete]
func (srv *MService) handleUserdel(c *gin.Context) {
	uid := c.Query("uid")
	if uid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uid is required"})
		return
	}

	err := srv.Minioth.Userdel(uid)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		} else if strings.Contains(err.Error(), "root") {
			c.JSON(400, gin.H{"error": "really bro?"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user"})
		}

		return
	}

	username, ok := c.Get("username")
	if !ok {
		username = "[debug]admin"
	}
	logAudit("user_del", username.(string), "user: "+uid, "success", "admin", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"message": "user deleted successfully"})
}

// @Summary Patch an existing user
// @Tags admin
// @Accept json
// @Produce json
// @Param data body map[string]any true "Fields to update (must include 'uid')"
// @Success 200 {object} map[string]string
// @Failure 400,404,500 {object} map[string]string
// @Router /admin/userpatch [patch]
func (srv *MService) handleUserpatch(c *gin.Context) {
	var updateFields map[string]any
	if err := c.ShouldBindJSON(&updateFields); err != nil {
		log.Printf("failed to bind req body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	uidValue, ok := updateFields["uid"]
	if !ok {
		log.Printf("uid is not ok: %v", uidValue)
		c.JSON(http.StatusBadRequest, gin.H{"error": "uid is required"})
		return
	}
	var uid string
	switch v := uidValue.(type) {
	case string:
		uid = v
	case float64:
		uid = fmt.Sprintf("%.0f", v)
	case int:
		uid = strconv.Itoa(v)
	default:
		log.Printf("uid type not supported: %T", v)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uid format"})
		return
	}

	if uid == "" {
		log.Print("empty uid")
		c.JSON(400, gin.H{"error": "uid was empty"})
		return
	}

	err := srv.Minioth.Userpatch(uid, updateFields)
	if err != nil {
		if strings.Contains(err.Error(), "WARNING") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		log.Printf("failed to patch user: %v", err)
		if err.Error() == "no inputs" {
			c.JSON(404, gin.H{"error": "bad request"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}

	username, ok := c.Get("username")
	if !ok {
		username = "[debug]admin"
	}
	logAudit("user_patch", username.(string), "user: "+uid, "success", "admin", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"message": "user patched successfully"})
}

// @Summary Modify an entire user entry
// @Tags admin
// @Accept json
// @Produce json
// @Param data body RegisterClaim true "User data for full update"
// @Success 200 {object} map[string]string
// @Failure 400,500 {object} map[string]string
// @Router /admin/usermod [put]
func (srv *MService) handleUsermod(c *gin.Context) {
	var ruser RegisterClaim
	if err := c.ShouldBindJSON(&ruser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := ruser.validateUser()
	if err != nil {
		log.Printf("invalid user, cannot update: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad input format"})
		return
	}

	err = srv.Minioth.Usermod(ruser.User)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}
	username, ok := c.Get("username")
	if !ok {
		username = "[debug]admin"
	}
	logAudit("user_mod", username.(string), "user: "+ruser.User.ToString(), "success", "admin", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

// @Summary Add a new group
// @Tags admin
// @Accept json
// @Produce json
// @Param data body ut.Group true "Group data"
// @Success 201 {object} map[string]string
// @Failure 400,500 {object} map[string]string
// @Router /admin/groupadd [post]
func (srv *MService) handleGroupadd(c *gin.Context) {
	var group ut.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		log.Printf("Invalid group data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group data"})
		return
	}

	if _, err := srv.Minioth.Groupadd(group); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "") {
			c.JSON(403, gin.H{"error": "already exists!"})
		} else {
			log.Printf("Failed to add group: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to insert the group",
			})
		}
		return
	}
	username, ok := c.Get("username")
	if !ok {
		username = "[debug]admin"
	}
	logAudit("group_mod", username.(string), "group: "+group.ToString(), "success", "admin", c.ClientIP())

	c.JSON(http.StatusCreated, gin.H{"message": "Group added successfully"})
}

// @Summary Patch an existing group
// @Tags admin
// @Accept json
// @Produce json
// @Param data body object{gid=string,fields=map[string]any} true "Fields to update in the group"
// @Success 200 {object} map[string]string
// @Failure 400,500 {object} map[string]string
// @Router /admin/grouppatch [patch]
func (srv *MService) handleGrouppatch(c *gin.Context) {
	var payload struct {
		Fields map[string]any `json:"fields" binding:"required"`
		Gid    string         `json:"gid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Printf("Invalid patch payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid patch payload"})
		return
	}

	if err := srv.Minioth.Grouppatch(payload.Gid, payload.Fields); err != nil {
		if strings.Contains(err.Error(), "WARNING") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		log.Printf("Failed to patch group: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to patch group"})
		return
	}

	username, ok := c.Get("username")
	if !ok {
		username = "[debug]admin"
	}
	logAudit("group_patch", username.(string), "group: "+payload.Gid, "success", "admin", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"message": "Group patched successfully"})
}

// @Summary Modify an entire group entry
// @Tags admin
// @Accept json
// @Produce json
// @Param data body ut.Group true "Full group object for replacement"
// @Success 200 {object} map[string]string
// @Failure 400,500 {object} map[string]string
// @Router /admin/groupmod [put]
func (srv *MService) handleGroupmod(c *gin.Context) {
	var group ut.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		log.Printf("Invalid group data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group data"})
		return
	}

	if err := srv.Minioth.Groupmod(group); err != nil {
		if strings.Contains(err.Error(), "WARNING") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		log.Printf("Failed to modify group: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to modify group"})
		return
	}

	username, ok := c.Get("username")
	if !ok {
		username = "[debug]admin"
	}
	logAudit("group_mod", username.(string), "group: "+group.ToString(), "success", "admin", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"message": "Group modified successfully"})
}

// @Summary Delete a group
// @Tags admin
// @Param gid query string true "Group ID to delete"
// @Success 200 {object} map[string]string
// @Failure 400,500 {object} map[string]string
// @Router /admin/groupdel [delete]
func (srv *MService) handleGroupdel(c *gin.Context) {
	gid := c.Query("gid")
	if gid == "" {
		log.Print("gid is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "gid is required"})
		return
	}

	if err := srv.Minioth.Groupdel(gid); err != nil {
		log.Printf("Failed to delete group: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
		return
	}
	username, ok := c.Get("username")
	if !ok {
		username = "[debug]admin"
	}
	logAudit("group_delete", username.(string), "group: "+gid, "success", "removed by admin", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"message": "Group deleted successfully"})
}

func logAudit(action, actor, target, status, details, source string) {
	mu.Lock()
	defer mu.Unlock()
	entry := map[string]any{
		"timestamp": ut.CurrentTime(),
		"action":    action,
		"actor":     actor,
		"target":    target,
		"status":    status,
		"source_ip": source,
		"details":   details,
	}
	data, _ := json.Marshal(entry)
	f, _ := os.OpenFile(audit_log_path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.Write(append(data, '\n'))
}
