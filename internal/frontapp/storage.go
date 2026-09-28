package frontendapp

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

/*
	Storage: files and volumes. Every call goes to uspace with the user's own
	token; uspace decides what the user may see and do.
*/

// volumeParam is the volume a request is about (the default one if unset).
func (srv *HTTPService) volumeParam(c *gin.Context) string {
	if v := c.Query("volume"); v != "" {
		return v
	}
	if v := c.GetHeader("X-Volume-Target"); v != "" {
		return v
	}

	return srv.Config.MinioDefaultBucket
}

func (srv *HTTPService) handleFetchResources(c *gin.Context) {
	volume := c.Query("volume")
	if volume == "" {
		volume = "*"
	}
	structType := c.DefaultQuery("struct", "list")
	resp, err := srv.uspace.do(c.Request.Context(), request{
		method: http.MethodGet, path: "/api/v1/resources", query: url.Values{"struct": {structType}},
		token: userToken(c), target: accessTarget(volume, "/"),
	})
	if err != nil {
		fail(c, "list files", err)

		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		fail(c, "list files", newUpstreamError(resp))

		return
	}

	if structType == "tree" {
		var data map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "unreadable file list"})

			return
		}
		c.HTML(http.StatusOK, "tree-resources.html", parseTreeNode("/", data))

		return
	}
	var data []ut.Resource
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "unreadable file list"})

		return
	}
	respondInFormat(c, c.Query("format"), data, "list-resources.html")
}

func (srv *HTTPService) handleResourceUpload(c *gin.Context) {
	resp, err := srv.uspace.do(c.Request.Context(), request{ //nolint:bodyclose // relay closes it
		method: http.MethodPost, path: "/api/v1/resource/upload",
		body: c.Request.Body, ctype: c.GetHeader("Content-Type"),
		token: userToken(c), target: accessTarget(srv.volumeParam(c), "/"),
	})
	if err != nil {
		fail(c, "upload", err)

		return
	}
	relay(c, resp)
}

func (srv *HTTPService) handleResourceDownload(c *gin.Context) {
	target := c.Query("target")
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a target"})

		return
	}
	resp, err := srv.uspace.do(c.Request.Context(), request{ //nolint:bodyclose // relay closes it
		method: http.MethodGet, path: "/api/v1/resource/download",
		token: userToken(c), target: accessTarget(srv.volumeParam(c), target),
		header: map[string]string{"Range": c.GetHeader("Range")},
	})
	if err != nil {
		fail(c, "download", err)

		return
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
		// quoted and escaped properly (the name used to be pasted between quotes)
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(target)}))
	}
	relay(c, resp, "Content-Range", "Accept-Ranges")
}

func (srv *HTTPService) handleResourcePreview(c *gin.Context) {
	name, rid := c.Query("resourcename"), c.Query("rid")
	if name == "" || rid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide resource name"})

		return
	}
	resp, err := srv.uspace.do(c.Request.Context(), request{
		method: http.MethodGet, path: "/api/v1/resource/preview", query: url.Values{"rid": {rid}},
		token: userToken(c), target: accessTarget(srv.volumeParam(c), name),
		header: map[string]string{"Range": c.GetHeader("Range")},
	})
	if err != nil {
		fail(c, "preview", err)

		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		fail(c, "preview", newUpstreamError(resp))

		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "preview interrupted"})

		return
	}
	c.Status(resp.StatusCode)
	respondInFormat(c, c.Query("format"), string(body), "resource-preview.html")
}

func (srv *HTTPService) handleResourceMove(c *gin.Context) {
	name := c.Query("resourcename")
	newName := c.PostForm("resourcename")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide resource name"})

		return
	}
	if err := ut.ValidateObjectName(newName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "should provide a proper new name"})

		return
	}
	volume := srv.volumeParam(c)
	resp, err := srv.uspace.do(c.Request.Context(), request{ //nolint:bodyclose // relay closes it
		method: http.MethodPatch, path: "/api/v1/resource/mv", query: url.Values{"dest": {volume + "/" + newName}},
		token: userToken(c), target: accessTarget(volume, name),
	})
	if err != nil {
		fail(c, "rename", err)

		return
	}
	relay(c, resp)
}

func (srv *HTTPService) handleResourceDelete(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing target parameter"})

		return
	}
	resp, err := srv.uspace.do(c.Request.Context(), request{ //nolint:bodyclose // relay closes it
		method: http.MethodDelete, path: "/api/v1/resource/rm",
		token: userToken(c), target: accessTarget(srv.volumeParam(c), name),
	})
	if err != nil {
		fail(c, "delete", err)

		return
	}
	relay(c, resp)
}

func (srv *HTTPService) handleResourceCopy(c *gin.Context) {
	name, dest := c.Query("resource"), c.Query("dest")
	if name == "" || dest == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide 'resource' and 'dest' (<volume>/<object>)"})

		return
	}
	resp, err := srv.uspace.do(c.Request.Context(), request{ //nolint:bodyclose // relay closes it
		method: http.MethodPost, path: "/api/v1/resource/cp", query: url.Values{"dest": {dest}},
		token: userToken(c), target: accessTarget(srv.volumeParam(c), name),
	})
	if err != nil {
		fail(c, "copy", err)

		return
	}
	relay(c, resp)
}

// handleResourcePerms changes a file's permissions, owner or group
// (uspace allows it to the file's owner and root).
func (srv *HTTPService) handleResourcePerms(c *gin.Context) {
	form := url.Values{}
	var endpoint string
	switch {
	case c.PostForm("owner") != "":
		endpoint = "/api/v1/resource/ownership"
		form.Set("owner", c.PostForm("owner"))
	case c.PostForm("group") != "":
		endpoint = "/api/v1/resource/group"
		form.Set("group", c.PostForm("group"))
	case c.PostForm("permissions") != "":
		endpoint = "/api/v1/resource/permissions"
		form.Set("permissions", c.PostForm("permissions"))
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid form field provided"})

		return
	}
	rid := c.Query("rid")
	if rid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty rid, must provide"})

		return
	}
	resp, err := srv.uspace.do(c.Request.Context(), request{ //nolint:bodyclose // relay closes it
		method: http.MethodPatch, path: endpoint, query: url.Values{"rid": {rid}},
		body: strings.NewReader(form.Encode()), ctype: "application/x-www-form-urlencoded",
		token: userToken(c), target: accessTarget(srv.volumeParam(c), "$rids="+rid),
	})
	if err != nil {
		fail(c, "change permissions", err)

		return
	}
	relay(c, resp)
}

// handleFetchVolumes lists the volumes the caller can use: every volume
// for admins; the default volume and their groups' shared volumes for
// users. (It used to list all volumes, as root, for everyone.)
func (srv *HTTPService) handleFetchVolumes(c *gin.Context) {
	admin := isAdmin(c)
	var volumes []ut.Volume
	if admin {
		var resp struct {
			Content []ut.Volume `json:"content"`
		}
		err := srv.uspace.decode(c.Request.Context(), request{
			method: http.MethodGet, path: "/api/v1/admin/volumes", token: userToken(c), target: "0::/",
		}, &resp)
		if err != nil {
			fail(c, "list volumes", err)

			return
		}
		volumes = resp.Content
	} else {
		var shared []ut.GroupVolume
		err := srv.uspace.decode(c.Request.Context(), request{
			method: http.MethodGet, path: "/api/v1/volumes/shared", token: userToken(c), target: "0::/",
		}, &shared)
		if err != nil {
			fail(c, "list volumes", err)

			return
		}
		volumes = append(volumes, ut.Volume{Name: srv.Config.MinioDefaultBucket})
		for _, gv := range shared {
			volumes = append(volumes, ut.Volume{VID: gv.VID, Name: gv.Vname, Capacity: gv.Quota, Usage: gv.Usage, Shared: true, GID: gv.GID})
		}
	}
	elevated := 0
	if admin {
		elevated = 1
	}
	respondInFormat(c, c.Query("format"), gin.H{"volumes": volumes, "elevated": elevated}, "volumes_template.html")
}

func (srv *HTTPService) handleVolumeadd(c *gin.Context) {
	var v ut.Volume
	if err := c.ShouldBind(&v); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad binding"})

		return
	}
	resp, err := srv.uspace.do(c.Request.Context(), request{ //nolint:bodyclose // relay closes it
		method: http.MethodPost, path: "/api/v1/admin/volumes", json: v, token: userToken(c), target: "0::/",
	})
	if err != nil {
		fail(c, "create volume", err)

		return
	}
	relay(c, resp)
}

func (srv *HTTPService) handleVolumedel(c *gin.Context) {
	vname := c.Query("volume")
	if vname == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing vname param"})

		return
	}
	resp, err := srv.uspace.do(c.Request.Context(), request{ //nolint:bodyclose // relay closes it
		method: http.MethodDelete, path: "/api/v1/admin/volumes", query: url.Values{"volume": {vname}},
		token: userToken(c), target: "0::/",
	})
	if err != nil {
		fail(c, "delete volume", err)

		return
	}
	relay(c, resp)
}

// isAdmin reports whether the session user is an admin (set by requireRole).
func isAdmin(c *gin.Context) bool {
	for _, g := range strings.Split(c.GetString("groups"), ",") {
		if strings.TrimSpace(g) == "admin" {
			return true
		}
	}

	return false
}
