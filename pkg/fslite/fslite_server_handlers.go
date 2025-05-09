package fslite

import (
	ut "kyri56xcaesar/myThesis/internal/utils"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (fsl *FsLite) loginHandler(c *gin.Context) {
	username := c.Request.URL.Query().Get("username")
	password := c.Request.URL.Query().Get("password")

	admin := Admin{}
	if password == "" || username == "" {
		err := c.BindJSON(&admin)
		if err != nil {
			log.Printf("failed to find query params and to bind...")
			c.JSON(http.StatusBadRequest, gin.H{"error": "must provide data"})
			return
		}
		username = admin.Username
		password = admin.Password
	}
	token, err := fsl.authenticateAdmin(username, password)
	if err != nil {
		log.Printf("failed to authenticate admin: %v", err)
		c.JSON(http.StatusForbidden, gin.H{"error": "failed to authenticate"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (fsl *FsLite) registerHandler(c *gin.Context) {
	username := c.Request.URL.Query().Get("username")
	password := c.Request.URL.Query().Get("password")

	admin := Admin{}
	if password == "" || username == "" {
		err := c.ShouldBind(&admin)
		if err != nil {
			log.Printf("failed to find query params and to bind...")
			c.JSON(http.StatusBadRequest, gin.H{"error": "must provide data"})
			return
		}
		username = admin.Username
		password = admin.Password
	}
	adm, err := fsl.insertAdmin(username, password)
	if err != nil {
		log.Printf("failed to insert admin user into the system")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "couldn't perform registration"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "admin registered", "uid": adm.ID})

}

func (fsl *FsLite) newVolumeHandler(c *gin.Context) {
	volume := ut.Volume{}
	err := c.BindJSON(&volume)
	if err != nil {
		log.Printf("error binding request body to struct: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	err = fsl.CreateVolume(volume)
	if err != nil {
		log.Printf("failed to create volume: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "volume created"})
}

func (fsl *FsLite) deleteVolumeHandler(c *gin.Context) {
	volume := ut.Volume{}
	err := c.BindJSON(&volume)
	if err != nil {
		log.Printf("error binding request body to struct: %v, trying query", err)
		vname := c.Request.URL.Query().Get("name")
		if vname == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "must provide volume body or name as url arg",
			})
			return
		}
		volume.Name = vname
	}

	err = fsl.RemoveVolume(volume)
	if err != nil {
		log.Printf("failed to delete volume: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "volume deleted"})
}

func (fsl *FsLite) getVolumeHandler(c *gin.Context) {
	vname := c.Request.URL.Query().Get("name")
	vid := c.Request.URL.Query().Get("vid")
	// format := c.Request.URL.Query().Get("format")

	volumes, err := fsl.SelectVolumes(map[string]any{"name": vname, "vid": vid})
	if err != nil {
		log.Printf("failed to get volume: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, volumes)
}

func (fsl *FsLite) statResourceHandler(c *gin.Context) {
	resource := c.Request.URL.Query().Get("resource")

	if resource == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide args"})
		return
	}
	parts := strings.Split(resource, "/")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "inv source format <volume_name>/<object_name>"})
		return
	}
	resource_vname := parts[0]
	resource_name := parts[1]

	res, err := fsl.Stat(ut.Resource{Vname: resource_vname, Name: resource_name}, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"content": res})

}

func (fsl *FsLite) getResourceHandler(c *gin.Context) {
	name := c.Request.URL.Query().Get("name")
	rids := c.Request.URL.Query().Get("rids")
	// format := c.Request.URL.Query().Get("format")

	resources, err := fsl.SelectObjects(map[string]any{"name": name, "rids": rids})
	if err != nil {
		if strings.Contains(err.Error(), "empty") {
			c.JSON(http.StatusNotFound, gin.H{"status": "empty"})
			return
		}
		log.Printf("failed to get resource: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resources)
}

func (fsl *FsLite) deleteResourceHandler(c *gin.Context) {
	resource := ut.Resource{}
	err := c.BindJSON(&resource)
	if err != nil || resource.Name == "" {
		log.Printf("error binding request body to struct: %v, trying query", err)
		name := c.Request.URL.Query().Get("name")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "must provide resource name as url arg or in body(json)",
			})
			return
		}
		vname := c.Request.URL.Query().Get("volume")
		if vname == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "must provide a volume name",
			})
			return
		}
		resource.Name = name
		resource.Vname = vname
	}

	err = fsl.Remove(resource)
	if err != nil {
		log.Printf("failed to delete resource: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "failed to delete the resource",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": resource.Vname + "/" + resource.Name + " deleted"})
}

func (fsl *FsLite) uploadResourceHandler(c *gin.Context) {
	id := c.Request.URL.Query().Get("uid")
	uid, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad uid, must specify"})
		return
	}
	vname := c.Request.URL.Query().Get("volume")
	if vname == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must specify volume"})
		return
	}

	err = c.Request.ParseMultipartForm(10 << 10)
	if err != nil {
		log.Printf("failed to parse multipart form: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse multipart form"})
		return
	}

	totalUploadSize := int64(0)
	for _, fileHeader := range c.Request.MultipartForm.File["files"] {
		totalUploadSize += fileHeader.Size
	}

	// claim userVolume, check if allowed!

	for _, fileHeader := range c.Request.MultipartForm.File["files"] {
		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("failed to read uploaded file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "fatal, failed to read uploaded files"})
			return
		}
		/* Insert the appropriate metadata as a resource */
		resource := ut.Resource{
			Vname:  vname,
			Name:   fileHeader.Filename,
			Type:   "file",
			Reader: file,
			Perms:  "rw-r--r--",
			Uid:    uid,
			Gid:    uid,
			Size:   int64(fileHeader.Size),
		}

		_, err = fsl.Insert(resource)
		file.Close()
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			log.Printf("failed to insert resources: %v", err)
			c.JSON(422, gin.H{"error": "failed to insert resources"})
			return
		}
		log.Printf("inserted resource: %+v", resource)
	}
	c.JSON(200, gin.H{
		"message": "file/s uploaded.",
	})
}

func (fsl *FsLite) downloadResourceHandler(c *gin.Context) {
	rsrc := c.Request.URL.Query().Get("resource")
	if rsrc == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide args"})
		return
	}
	parts := strings.Split(rsrc, "/")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "inv source format <volume_name>/<object_name>"})
		return
	}
	resource_vname := parts[0]
	resource_name := parts[1]
	if resource_vname == "" || resource_name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "inv source format <volume_name>/<object_name>"})
		return
	}

	resource := any(ut.Resource{Name: resource_name, Vname: resource_vname})
	_, err := fsl.Download(&resource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to download"})
		return
	}
	r, ok := resource.(ut.Resource)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to download"})
		return
	}
	if r.Reader == nil {
		log.Printf("resource reader is nil")
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		return
	}
	c.DataFromReader(http.StatusOK, r.Size, r.Name, r.Reader, map[string]string{
		"Content-Disposition": "attachment; filename=\"" + r.Name + "\"",
		"Content-Type":        "application/octet-stream",
	})
}

func (fsl *FsLite) copyResourceHandler(c *gin.Context) {
	src := c.Request.URL.Query().Get("source")
	dst := c.Request.URL.Query().Get("dest")

	if src == "" || dst == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide args"})
		return
	}
	parts := strings.Split(src, "/")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "inv source format <volume_name>/<object_name>"})
		return
	}
	src_vname := parts[0]
	src_name := parts[1]

	parts = strings.Split(dst, "/")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "inv source format <volume_name>/<object_name>"})
		return
	}
	dst_vname := parts[0]
	dst_name := parts[1]

	err := fsl.Copy(ut.Resource{Name: src_name, Vname: src_vname}, ut.Resource{Name: dst_name, Vname: dst_vname})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to copy the objs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "copy complete"})
}

func (fsl *FsLite) shareResourceHandler(c *gin.Context) {

}

func (fsl *FsLite) handleUserVolumes(c *gin.Context) {
	// limit, err := strconv.Atoi(c.Request.URL.Query().Get("limit"))
	// if err != nil {
	// 	limit = 0
	// }
	uids := c.Request.URL.Query().Get("uids")
	rids := c.Request.URL.Query().Get("vids")

	switch c.Request.Method {
	case http.MethodGet:
		res, err := fsl.selectUserVolumes(map[string]any{"uids": uids, "rids": rids})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user volumes"})
		} else {
			c.JSON(http.StatusOK, gin.H{"content": res})
		}
	case http.MethodPatch:
	case http.MethodDelete:

	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "method not allowed"})
	}
}
