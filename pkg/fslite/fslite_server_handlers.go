// Package fslite details
//
// @title           Fslite API
// @version         1.0
// @description     http server API for basic volume/object management (on filesystem) with user claim provisioning.
// @host            localhost:7070 (config)
// @BasePath        /
// @schemes         http
package fslite

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

// @Summary Admin login
// @Description Authenticates an admin user and returns a token for access.
// @Tags auth
// @Accept json
// @Produce json
// @Param username query string false "Admin username (optional if provided in body)"
// @Param password query string false "Admin password (optional if provided in body)"
// @Param admin body Admin false "Admin credentials in request body (optional)"
// @Success 200 {object} map[string]string "Token returned on successful authentication"
// @Failure 400 {object} map[string]string "Missing or invalid input"
// @Failure 403 {object} map[string]string "Authentication failed"
// @Router /login [post]
func (fsl *FsLite) loginHandler(c *gin.Context) {
	username := c.Request.URL.Query().Get("username")
	password := c.Request.URL.Query().Get("password")

	admin := Admin{}
	if password == "" || username == "" {
		err := c.BindJSON(&admin)
		if err != nil {
			log.Printf("[FSL_API_login] failed to find query params and to bind...")
			c.JSON(http.StatusBadRequest, gin.H{"error": "must provide data"})

			return
		}
		username = admin.Username
		password = admin.Password
	}
	token, err := fsl.authenticateAdmin(username, password)
	if err != nil {
		log.Printf("[FSL_API_login] failed to authenticate admin: %v", err)
		c.JSON(http.StatusForbidden, gin.H{"error": "failed to authenticate"})

		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// @Summary Admin registration
// @Description Registers a new admin user into the system.
// @Tags auth
// @Accept json
// @Produce json
// @Param username query string false "Admin username (optional if provided in body)"
// @Param password query string false "Admin password (optional if provided in body)"
// @Param admin body Admin false "Admin registration data in body (optional)"
// @Success 201 {object} map[string]interface{} "Admin registered successfully"
// @Failure 400 {object} map[string]string "Missing or invalid input"
// @Failure 500 {object} map[string]string "Server error during registration"
// @Router /admin/register [post]
func (fsl *FsLite) registerHandler(c *gin.Context) {
	username := c.Request.URL.Query().Get("username")
	password := c.Request.URL.Query().Get("password")

	admin := Admin{}
	if password == "" || username == "" {
		err := c.ShouldBind(&admin)
		if err != nil {
			log.Printf("[FSL_API_register] failed to find query params and to bind...")
			c.JSON(http.StatusBadRequest, gin.H{"error": "must provide data"})

			return
		}
		username = admin.Username
		password = admin.Password
	}
	adm, err := fsl.insertAdmin(username, password)
	if err != nil {
		log.Printf("[FSL_API_register] failed to insert admin user into the system")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "couldn't perform registration"})

		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "admin registered", "uid": adm.ID})
}

// @Summary Create a new volume
// @Description Registers a new volume with specified metadata.
// @Tags volume
// @Accept json
// @Produce json
// @Param volume body ut.Volume true "Volume object"
// @Success 200 {object} map[string]string "volume created"
// @Failure 400 {object} map[string]string "binding or creation error"
// @Router /admin/volume/new [post]
func (fsl *FsLite) newVolumeHandler(c *gin.Context) {
	volume := ut.Volume{}
	err := c.BindJSON(&volume)
	if err != nil {
		log.Printf("[FSL_API_newVolume] error binding request body to struct: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})

		return
	}
	volume.Capacity = min(volume.Capacity, maxVolumeCap)
	if volume.Capacity == 0 {
		volume.Capacity = maxVolumeCap
	}

	err = fsl.CreateVolume(volume)
	if err != nil {
		log.Printf("[FSL_API_newVolume] failed to create volume: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})

		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "volume created"})
}

// @Summary Delete a volume
// @Description Deletes a volume either from JSON body or query param `name`.
// @Tags volume
// @Accept json
// @Produce json
// @Param volume body ut.Volume false "Volume object"
// @Param name query string false "Volume name"
// @Success 200 {object} map[string]string "volume deleted"
// @Failure 400 {object} map[string]string "deletion error or invalid input"
// @Router /admin/volume/delete [delete]
func (fsl *FsLite) deleteVolumeHandler(c *gin.Context) {
	volume := ut.Volume{}
	err := c.BindJSON(&volume)
	if err != nil {
		log.Printf("[FSL_API_delVolume] error binding request body to struct: %v, trying query", err)
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
		log.Printf("[FSL_API_delVolume] failed to delete volume: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})

		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "volume deleted"})
}

// @Summary Get volume information
// @Description Retrieves volume details using query parameters.
// @Tags volume
// @Produce json
// @Param name query string false "Volume name"
// @Param vid query string false "Volume ID"
// @Success 200 {object} []ut.Volume "volume info"
// @Failure 400 {object} map[string]string "retrieval error"
// @Router /admin/volume/get [get]
func (fsl *FsLite) getVolumeHandler(c *gin.Context) {
	vname := c.Request.URL.Query().Get("name")
	vid := c.Request.URL.Query().Get("vid")
	// format := c.Request.URL.Query().Get("format")

	volumes, err := fsl.SelectVolumes(map[string]any{"name": vname, "vid": vid})
	if err != nil {
		log.Printf("[FSL_API_getVolume] failed to get volume: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})

		return
	}
	c.JSON(http.StatusOK, volumes)
}

// @Summary Get metadata of a resource
// @Description Retrieves the metadata of a specific resource using its volume and name.
// @Tags resource
// @Produce json
// @Param resource query string true "Format: <volume_name>/<object_name>"
// @Success 200 {object} map[string]interface{} "Resource metadata"
// @Failure 400 {object} map[string]string "Invalid input or formatting"
// @Failure 500 {object} map[string]string "Server error during stat"
// @Router /admin/resource/stat [get]
func (fsl *FsLite) statResourceHandler(c *gin.Context) {
	resource := c.Request.URL.Query().Get("resource")

	if resource == "" {
		log.Printf("[FSL_API_statResource] no resource argument provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide args"})

		return
	}
	parts := strings.Split(resource, "/")
	if len(parts) != 2 {
		log.Printf("[FSL_API_statResource] resource argument bad format")
		c.JSON(http.StatusBadRequest, gin.H{"error": "inv source format <volume_name>/<object_name>"})

		return
	}
	resourceVname := parts[0]
	resourceName := parts[1]

	res, err := fsl.Stat(ut.Resource{Vname: resourceVname, Name: resourceName})
	if err != nil {
		log.Printf("[FSL_API_statResource] failed to stat resource")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stat"})

		return
	}

	c.JSON(http.StatusOK, gin.H{"content": res})
}

// @Summary Get one or more resources
// @Description Retrieves resources using name or resource IDs (rids).
// @Tags resource
// @Produce json
// @Param name query string false "Resource name to search"
// @Param rids query string false "Comma-separated list of resource IDs"
// @Success 200 {array} ut.Resource "List of resources"
// @Failure 400 {object} map[string]string "Query or processing error"
// @Failure 404 {object} map[string]string "No matching resources found"
// @Router /admin/resource/get [get]
func (fsl *FsLite) getResourceHandler(c *gin.Context) {
	name := c.Request.URL.Query().Get("name")
	rids := c.Request.URL.Query().Get("rids")
	// format := c.Request.URL.Query().Get("format")

	resources, err := fsl.SelectObjects(map[string]any{"prefix": name, "rids": rids})
	if err != nil {
		if strings.Contains(err.Error(), "empty") {
			c.JSON(http.StatusNotFound, gin.H{"status": "empty"})

			return
		}
		log.Printf("[FSL_API_getResource] failed to get resource: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, resources)
}

// @Summary Delete a resource
// @Description Deletes a resource by JSON body or query params (name & volume).
// @Tags resource
// @Accept json
// @Produce json
// @Param resource body ut.Resource false "Resource to delete"
// @Param name query string false "Name of the resource"
// @Param volume query string false "Volume name of the resource"
// @Success 200 {object} map[string]string "Successful deletion"
// @Failure 400 {object} map[string]string "Bad request or invalid input"
// @Failure 500 {object} map[string]string "Server error or internal failure"
// @Router /admin/resource/delete [delete]
func (fsl *FsLite) deleteResourceHandler(c *gin.Context) {
	id, ok := c.Get("uid")
	uid, ok2 := id.(string)
	if !ok || !ok2 {
		if strings.ToLower(fsl.config.APIGinMode) == "debug" {
			log.Printf("[FSL_API_delResource] mode = [debug], entering default uid")
			uid = "0"
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "middleware failed to authenticate"})

			return
		}
	}
	resource := ut.Resource{}
	err := c.BindJSON(&resource)
	if err != nil || resource.Name == "" {
		log.Printf("[FSL_API_delResource] error binding request body to struct: %v, trying query", err)
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

	if !unlocked {
		res, err := fsl.SelectObjects(map[string]any{"name": resource.Name, "volume": resource.Vname})
		if err != nil {
			log.Printf("failed to retrieve info for the specified object")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the object info"})

			return
		}
		resources, ok := res.([]ut.Resource)
		if !ok {
			r, ok := res.(ut.Resource)
			if !ok {
				log.Printf("failed to cast the result as resource and as resource slice")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "bad state"})

				return
			}
			err = fsl.releaseVolumeSpace(r.Size, r.Vname, uid)
			if err != nil {
				log.Printf("failed to release the volume claim due to deletion: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to release volume claim"})

				return
			}

		} else {
			err = fsl.releaseVolumeSpace(resources[0].Size, resource.Vname, uid)
			if err != nil {
				log.Printf("failed to release the volume claim due to deletion: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to release volume claim"})

				return
			}
		}
	}

	err = fsl.Remove(resource)
	if err != nil {
		log.Printf("failed to delete resource: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "failed to delete the resource",
		})

		return
	}
	// release volume claim

	c.JSON(http.StatusOK, gin.H{"status": resource.Vname + "/" + resource.Name + " deleted"})
}

// @Summary Upload one or more files to a volume
// @Description Uploads multiple files to the specified volume. Requires authentication.
// @Tags resource
// @Accept multipart/form-data
// @Produce json
// @Param volume query string true "Volume name"
// @Param files formData file true "Files to upload" multiple
// @Success 200 {object} map[string]string "Files uploaded successfully"
// @Failure 400 {object} map[string]string "Bad request or parse failure"
// @Failure 422 {object} map[string]string "Failed to insert resource"
// @Router /admin/resource/upload [post]
func (fsl *FsLite) uploadResourceHandler(c *gin.Context) {
	id, ok := c.Get("uid")
	uidStr, ok2 := id.(string)
	uid, err := strconv.Atoi(uidStr)
	if !ok || !ok2 || err != nil {
		log.Printf("[FSL_API_uploadResource] uid wasn't set properly.")
		if strings.ToLower(fsl.config.APIGinMode) == "debug" {
			log.Printf("mode = [debug], entering default uid")
			uid = 0
			uidStr = "0"
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "middleware failed to authenticate"})

			return
		}
	}
	vname := c.Request.URL.Query().Get("volume")
	if vname == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must specify volume"})

		return
	}

	err = c.Request.ParseMultipartForm(10 << 10)
	if err != nil {
		log.Printf("[FSL_API_uploadResource] failed to parse multipart form: %v", err)
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
			UID:    uid,
			GID:    uid,
			Size:   fileHeader.Size,
		}

		if !unlocked {
			err = fsl.claimVolumeSpace(resource.Size, resource.Vname, uidStr)
			if err != nil {
				err = file.Close()
				if err != nil {
					log.Printf("failed to close the file: %v", err)
				}
				log.Printf("failed to claim volume space.: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "volume claim denied"})

				return
			}
		}

		_, err = fsl.Insert(resource)
		if err != nil {
			log.Printf("failed to insert the resource: %v", err)
		}
		err = file.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				c.JSON(400, gin.H{"error": err.Error()})

				return
			}
			log.Printf("failed to insert resources: %v", err)
			c.JSON(422, gin.H{"error": "failed to insert resources"})

			return
		}
		if verbose {
			log.Printf("inserted resource: %+v", resource)
		}
	}
	c.JSON(200, gin.H{
		"message": "file/s uploaded.",
	})
}

// @Summary Download a resource
// @Description Downloads a resource from a specified volume by filename.
// @Tags resource
// @Produce application/octet-stream
// @Param resource query string true "Format: <volume_name>/<object_name>"
// @Success 200 {file} file "File stream for download"
// @Failure 400 {object} map[string]string "Invalid request format"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /admin/resource/download [get]
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
	resourceVname := parts[0]
	resourceName := parts[1]
	if resourceVname == "" || resourceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "inv source format <volume_name>/<object_name>"})

		return
	}

	resource := any(ut.Resource{Name: resourceName, Vname: resourceVname})
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
		log.Printf("[FSL_API_downloadResource] resource reader is nil")
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})

		return
	}
	c.DataFromReader(http.StatusOK, r.Size, r.Name, r.Reader, map[string]string{
		"Content-Disposition": "attachment; filename=\"" + r.Name + "\"",
		"Content-Type":        "application/octet-stream",
	})
}

// @Summary Copy a resource
// @Description Copies a resource from one location to another within the system.
// @Tags resource
// @Produce json
// @Param source query string true "Source resource in format <volume_name>/<object_name>"
// @Param dest query string true "Destination resource in format <volume_name>/<object_name>"
// @Success 200 {object} map[string]string "Copy successful"
// @Failure 400 {object} map[string]string "Invalid format or missing arguments"
// @Failure 500 {object} map[string]string "Server error"
// @Router /admin/resource/copy [post]
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
	srvVname := parts[0]
	srcName := parts[1]

	parts = strings.Split(dst, "/")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "inv source format <volume_name>/<object_name>"})

		return
	}
	dstVname := parts[0]
	dstName := parts[1]

	err := fsl.Copy(ut.Resource{Name: srcName, Vname: srvVname}, ut.Resource{Name: dstName, Vname: dstVname})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to copy the objs"})

		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "copy complete"})
}

func (fsl *FsLite) shareResourceHandler(_ *gin.Context) {
}

// @Summary Manage user volume claims
// @Description Fetch user-to-volume mappings. (Currently only GET is implemented. PATCH/DELETE placeholders.)
// @Tags volume
// @Produce json
// @Param uids query string false "Comma-separated user IDs to filter"
// @Param vids query string false "Comma-separated volume IDs to filter"
// @Success 200 {object} map[string]any "User volume claims retrieved"
// @Failure 500 {object} map[string]string "Server error"
// @Failure 403 {object} map[string]string "Method not allowed"
// @Router /admin/uservolumes [get]
func (fsl *FsLite) handleUserVolumes(c *gin.Context) {
	// limit, err := strconv.Atoi(c.Request.URL.Query().Get("limit"))
	// if err != nil {
	// 	limit = 0
	// }
	uids := c.Request.URL.Query().Get("uids")
	vids := c.Request.URL.Query().Get("vids")

	switch c.Request.Method {
	case http.MethodGet:
		res, err := fsl.selectUserVolumes(map[string]any{"uids": uids, "vids": vids})
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

func (fsl *FsLite) handleSysConf(c *gin.Context) {
	fslcfg, err := ut.ReadConfig("configs/"+fsl.config.ConfigPath, false)
	if err != nil {
		log.Printf("[API_sysConf] failed to read config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}
	c.JSON(http.StatusOK, fslcfg)
}
