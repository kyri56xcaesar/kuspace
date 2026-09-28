package frontendapp

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

/*
	Pages and system views: the panel itself, the system configuration and
	metrics (admins), plus the helpers the handlers share.
*/

func (srv *HTTPService) handleAdminPanel(c *gin.Context) {
	var me ut.User
	if err := srv.minioth.decode(c.Request.Context(), request{method: http.MethodGet, path: "/v1/user/me", token: userToken(c)}, &me); err != nil {
		fail(c, "load your profile", err)

		return
	}
	elevated := 0
	if isAdmin(c) {
		elevated = 1
	}
	c.HTML(http.StatusOK, "admin-panel.html", gin.H{
		"username": c.GetString("username"),
		"message":  "Welcome to the Admin Panel ",
		"info":     me.Info,
		"home":     me.Home,
		"elevated": elevated,
		"groups":   c.GetString("groups"),
		"uid":      me.UID,
		// per-user default quota (GB) that registration claims for every user
		"quotaGB": srv.Config.LocalVolumesDefaultCapacity,
	})
}

// handleSysConf shows the (secret-free) configuration of the services.
func (srv *HTTPService) handleSysConf(c *gin.Context) {
	services := c.Query("services")
	if services == "" || services == "*" {
		services = "uspace,wss,frontapp" // minioth has no system-conf endpoint
	}
	ctx := c.Request.Context()
	out := map[string]map[string]string{}
	for _, service := range strings.Split(services, ",") {
		var cfg map[string]string
		var err error
		switch strings.TrimSpace(service) {
		case "uspace":
			err = srv.uspace.decode(ctx, request{method: http.MethodGet, path: "/api/v1/admin/system-conf", token: userToken(c), target: "0::/"}, &cfg)
		case "wss":
			err = srv.wss.decode(ctx, request{method: http.MethodGet, path: "/system-conf", svc: true}, &cfg)
		case "frontapp":
			cfg, err = ut.ReadConfig("configs/"+srv.Config.ConfigPath, false)
		default:
			continue
		}
		if err != nil {
			log.Printf("[frontapp] system-conf of %s: %v", service, err)
			cfg = map[string]string{"error": "unavailable"}
		}
		out[strings.TrimSpace(service)] = cfg
	}
	respondInFormat(c, c.Query("format"), out, "sys_conf_display.html")
}

func (srv *HTTPService) handleSysMetrics(c *gin.Context) {
	var metrics map[string]any
	if err := srv.uspace.decode(c.Request.Context(), request{
		method: http.MethodGet, path: "/api/v1/admin/system-metrics", token: userToken(c), target: "0::/",
	}, &metrics); err != nil {
		fail(c, "load metrics", err)

		return
	}
	respondInFormat(c, c.Query("format"), metrics, "metrics_display.html")
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

// TreeNode struct describes the "set" of resources in a tree like representation
// using maps
type TreeNode struct {
	Resource *ut.Resource         `json:"resource,omitempty"`
	Children map[string]*TreeNode `json:"children,omitempty"`
	Name     string               `json:"name,omitempty"`
	Type     string               `json:"type"` // "directory" or "file"
}
