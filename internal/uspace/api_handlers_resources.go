package uspace

/*
	http api handlers for the uspace service
	"resource" related endpoints
*/

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

/* perhaps move here all the  Resource specific funcs/methods/structs/models...*/
// @SEE models.go

/* HTTP Gin handlers related to resources, used by the api to handle endpoints*/

/*
* this should behave as:
* 'ls'
* */
//
// @Summary     List resources (like `ls`)
// @Description Returns resources for a given access context. Supports list, tree, or content views.
// @Tags        resources
// @Accept      json
// @Produce     json
//
// @Param       limit    query     int     false  "Maximum number of results"
// @Param       struct   query     string  false  "Output structure: list, tree, or content"  Enums(list,tree,content)
//
// @Success     200      {object}  map[string]interface{}  "Resources or resource tree"
// @Failure     404      {object}  map[string]string       "No resources found"
// @Failure     500      {object}  map[string]string       "Internal server error"
//
// @Router      /resources [get]
func (srv *UService) getResourcesHandler(c *gin.Context) {
	l := c.Request.URL.Query().Get("limit")
	limit, err := strconv.Atoi(l)
	if err != nil {
		limit = 0
	}

	// get header
	acH, exists := c.Get("accessTarget")
	if !exists {
		log.Printf("access target header was not set properly")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "access target header was not set correctly"})

		return
	}
	ac := acH.(ut.AccessClaim)
	// prefer to get from db // must be ensure its in sync
	res, err := srv.fsl.SelectObjects(
		map[string]any{
			"vname":  ac.Vname,
			"prefix": strings.TrimPrefix(ac.Target, "/"),
			"limit":  limit,
		},
	)
	if err != nil {
		if strings.Contains(err.Error(), "scan") || strings.Contains(err.Error(), "empty") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		} else {
			log.Printf("failed to retrieve objects from storage system: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "fatal"})
		}

		return
	}

	resources, ok := res.([]ut.Resource)
	if !ok {
		log.Printf("error retrieving resource: %v", res)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bad format"})

		return
	} else if resources == nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "no objects found"})

		return
	}

	// we should determine the structure to be returned.
	// this is given as uri argument
	// default is list
	structure := c.Request.URL.Query().Get("struct")

	switch structure {
	case "list":
		c.JSON(200, resources)
	case "tree":
		// build the tree in json
		// need to parse all the resources
		tree := make(map[string]any)
		for _, resource := range resources {
			buildTreeRec(tree, append([]string{"/"}, strings.Split(strings.TrimPrefix(resource.Name, "/"), "/")...), resource)
		}

		c.JSON(200, tree)
	case "content":
		c.JSON(200, gin.H{"content": resources})
	default:
		c.JSON(200, resources)
	}
}

// @Summary     Delete a resource
// @Description Deletes the resource specified by the access claim in the request context.
// @Tags        resources
// @Accept      json
// @Produce     json
//
// @Success     200  {object}  map[string]string  "Resource deleted successfully"
// @Failure     500  {object}  map[string]string  "Internal server error or access claim missing"
//
// @Router      /resource/rm [delete]
func (srv *UService) rmResourceHandler(c *gin.Context) {
	// its assumed that the user is privelleged to download (from middleware) (write)
	// get header
	acH, exists := c.Get("accessTarget")
	if !exists {
		log.Printf("access target header was not set properly")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "access target header was not set correctly"})

		return
	}
	ac := acH.(ut.AccessClaim)

	// if this fails perhaps we need to delete the db entry...
	if err := srv.storage.Remove(ut.Resource{
		Name:  ac.Target,
		Vname: ac.Vname,
	}); err != nil {
		log.Printf("error when removing object: %v", err) // perhaps specify exact details of error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete obj"})

		return
	}

	if err := srv.fsl.Remove(ut.Resource{
		Name:  ac.Target,
		Vname: ac.Vname,
	}); err != nil {
		log.Printf("error when removing object from fsl: %v", err) // perhaps specify exact details of error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete obj"})

		return
	}

	c.JSON(200, gin.H{
		"message": "resource deleted successfully.",
	})
}

// @Summary     Move a resource
// @Description Moves a resource (copy + delete) to another bucket/object path using the `dest` query parameter.
// @Tags        resources
// @Accept      json
// @Produce     json
//
// @Param       dest  query     string  true  "Destination path in 'bucket/object' format"
//
// @Success     200   {object}  map[string]string  "Resource moved successfully"
// @Failure     400   {object}  map[string]string  "Invalid destination format or missing destination"
// @Failure     500   {object}  map[string]string  "Copy or delete failed"
//
// @Router      /resource/mv [post]
func (srv *UService) mvResourcesHandler(c *gin.Context) {
	// its assumed that the user is privelleged to download (from middleware) (write)
	// get header
	acH, exists := c.Get("accessTarget")
	if !exists {
		log.Printf("access target header was not set properly")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "access target header was not set correctly"})

		return
	}
	ac := acH.(ut.AccessClaim)

	dest := c.Request.URL.Query().Get("dest")
	if dest == "" {
		log.Printf("request doesn't provide 'dest'")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must specify destination 'dest'"})

		return
	}

	parts := strings.SplitN(dest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		log.Printf("dest is in bad format")
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad destination format 'bucket/object'"})

		return
	}

	if err := srv.storage.Copy(
		ut.Resource{
			Name:  ac.Target,
			Vname: ac.Vname,
		},
		ut.Resource{
			Name:  parts[1],
			Vname: parts[0],
		},
	); err != nil {
		log.Printf("failed to make actual copy: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to copy object"})

		return
	}

	if err := srv.storage.Remove(
		ut.Resource{
			Name:  ac.Target,
			Vname: ac.Vname,
		},
	); err != nil {
		log.Printf("failed to delete the actual old file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete original"})

		return
	}

	// database
	err := srv.fsl.Update(map[string]string{"newname": parts[1], "volume": parts[0], "name": ac.Target})
	if err != nil {
		log.Printf("failed to update inner fsl: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update file in local db"})

		return
	}

	c.JSON(200, gin.H{
		"message": "resource moved successfully",
	})
}

// @Summary     Copy a resource
// @Description Copies a resource from one path to another using the `dest` query param in `bucket/object` format.
// @Tags        resources
// @Accept      json
// @Produce     json
//
// @Param       dest  query     string  true  "Destination path in 'bucket/object' format"
//
// @Success     200   {object}  map[string]string  "Successful copy"
// @Failure     400   {object}  map[string]string  "Bad request or missing destination"
// @Failure     500   {object}  map[string]string  "Copy failed or internal error"
//
// @Router      /resource/cp [post]
func (srv *UService) cpResourceHandler(c *gin.Context) {
	// its assumed that the user is privelleged to download (from middleware) read
	// get header
	acH, exists := c.Get("accessTarget")
	if !exists {
		log.Printf("access target header was not set properly")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "access target header was not set correctly"})

		return
	}
	ac := acH.(ut.AccessClaim)

	dest := c.Request.URL.Query().Get("dest")
	if dest == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must specify destination 'dest'"})

		return
	}

	parts := strings.SplitN(dest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad destination format 'bucket/object'"})

		return
	} // destV := parts[0] //destN := parts[1]

	// database
	if err := srv.fsl.Copy(
		ut.Resource{
			Name:  ac.Target,
			Vname: ac.Vname,
		},
		ut.Resource{
			Name:  parts[1],
			Vname: parts[0],
		},
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to copy object"})

		return
	}

	if err := srv.storage.Copy(
		ut.Resource{
			Name:  ac.Target,
			Vname: ac.Vname,
		},
		ut.Resource{
			Name:  parts[1],
			Vname: parts[0],
		},
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to copy object"})

		return
	}

	c.JSON(200, gin.H{"message": "successful copy"})
}

// @Summary     Download a resource
// @Description Downloads a single resource specified by the access context (must be a file, not a directory).
// @Tags        resources
// @Accept      json
// @Produce     application/octet-stream
//
// @Success     200  {file}    binary  "File downloaded"
// @Failure     400  {object}  map[string]string  "Bad request (e.g., target is a directory or vid is invalid)"
// @Failure     404  {object}  map[string]string  "Resource not found"
// @Failure     500  {object}  map[string]string  "Failed to access or stream the resource"
//
// @Router      /resource/download [get]
func (srv *UService) handleDownload(c *gin.Context) {
	// its assumed that the user is privelleged to download (from middleware)
	/* 1]: parse location from header*/
	// get header
	acH, exists := c.Get("accessTarget")
	if !exists {
		log.Printf("access target header was not set properly")
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": "access target header was not set correctly"})

		return
	}
	ac := acH.(ut.AccessClaim)

	if strings.HasSuffix(ac.Target, "/") {
		log.Printf("target should be a file, not a directory")
		c.JSON(http.StatusBadRequest, gin.H{"error": "target should be a file, not a directory"})

		return
	}
	// get the path
	parts := strings.Split(ac.Target, "/")
	path := strings.Join(parts[1:], "/")
	name := parts[len(parts)-1]
	vid, err := strconv.Atoi(ac.VID)
	if err != nil && ac.Vname == "" {
		log.Printf("failed to atoi vid: %v and vname not provided", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad vid"})

		return
	}

	resource := &ut.Resource{
		Path:  path,
		Name:  name,
		Vname: ac.Vname,
		VID:   vid,
		Size:  -1,
	}
	var aR any = resource

	cancelFn, err := srv.storage.Download(&aR)
	if err != nil {
		log.Printf("failed to retrieve resource: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to retrieve resource"})

		return
	}
	defer cancelFn()

	if resource.Reader == nil {
		log.Printf("resource reader is nil")
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})

		return
	}

	c.DataFromReader(http.StatusOK, resource.Size, resource.Name, resource.Reader,
		map[string]string{
			"Content-Disposition": "attachment; filename=\"" + resource.Name + "\"",
			"Content-Type":        "application/octet-stream",
		})
}

/* the main endpoint handler for resource uploading */
//
// @Summary     Upload files
// @Description Uploads one or more files to a specified resource path from multipart form data.
// @Tags        resources
// @Accept      multipart/form-data
// @Produce     json
//
// @Param       files  formData  file  true  "Files to upload (multiple supported)"
//
// @Success     200    {object}  map[string]string  "Files uploaded successfully"
// @Failure     400    {object}  map[string]string  "Bad request or form error"
// @Failure     422    {object}  map[string]string  "Unprocessable entity (insertion error)"
// @Failure     500    {object}  map[string]string  "Internal server error"
//
// @Router      /resource/upload [post]
func (srv *UService) handleUpload(c *gin.Context) {
	/* 1]: parse location from header*/
	// get header
	acH, exists := c.Get("accessTarget")
	if !exists {
		log.Printf("access target header was not set properly")
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": "access target header was not set correctly"})

		return
	}
	ac, ok := acH.(ut.AccessClaim)
	if !ok {
		log.Printf("failed to cast, ac wasn't set properly")
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": "access target wasn't set properly"})

		return
	}

	// 2]: authorization should be checked by now, by a middleware
	/*
	* */

	// 3]: determine physical destination path
	// parse the form files
	err := c.Request.ParseMultipartForm(10 << 10)
	if err != nil {
		log.Printf("failed to parse multipart form: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse multipart form"})

		return
	}

	/* physical path should be the target path given.
	 * This function will also perform some checks
	 */
	// lets calc the total size as well, prematurely.
	totalUploadSize := int64(0)
	for _, fileHeader := range c.Request.MultipartForm.File["files"] {
		totalUploadSize += fileHeader.Size
	}

	// 4]: perform the upload stream
	/* I would like to do this concurrently perpahps*/
	for _, fileHeader := range c.Request.MultipartForm.File["files"] {
		uid, err := strconv.Atoi(ac.UID)
		if err != nil {
			log.Printf("failed to atoi uid: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad uid"})

			return
		}
		vid, err := strconv.Atoi(ac.VID)
		if err != nil {
			log.Printf("failed to atoi vid: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad vid"})

			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("failed to read uploaded file: %v", err)
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": "fatal, failed to read uploaded files"})

			return
		}
		defer func() {
			err := file.Close()
			if err != nil {
				log.Printf("failed to close file: %v", err)
			}
		}()

		currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
		/* Insert the appropriate metadata as a resource */
		resource := ut.Resource{
			VID:    vid,
			Vname:  ac.Vname,
			Name:   ac.Target + fileHeader.Filename,
			Path:   ac.Target,
			Type:   "file",
			Reader: file,

			CreatedAt:  currentTime,
			UpdatedAt:  currentTime,
			AccessedAt: currentTime,
			Perms:      "rw-r--r--",
			UID:        uid,
			GID:        uid,
			Size:       fileHeader.Size,
		}

		_, err = srv.storage.Insert(resource)
		if err != nil {
			log.Printf("failed to insert resources: %v", err)
			c.JSON(422, gin.H{"error": "failed to insert resources"})

			return
		}
		// defer cancelFn()
		_, err = srv.fsl.Insert(resource)
		if err != nil {
			log.Printf("failed to insert resources to db: %v", err)
			c.JSON(422, gin.H{"error": "failed to insert resources"})

			return
		}
	}
	c.JSON(200, gin.H{
		"message": "file/s uploaded.",
	})
}

// handlePreview provides partial content preview of a resource.
//
// @Summary Preview a resource
// @Description Returns a byte-range preview (default 4KB) of a file resource. Assumes user is authorized.
// @Tags resources
//
// @Produce text/plain
//
// @Param Range header string false "Byte range to preview (e.g., 'bytes=0-4095')"
// @Success 206 {string} string "Partial content"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 416 {object} map[string]string "Requested range exceeds file size"
// @Failure 500 {object} map[string]string "Internal server error"
//
// @Router /resource/preview [get]
func (srv *UService) handlePreview(c *gin.Context) {
	// its assumed that the user is privelleged to download (from middleware) (read)

	// parse resource target header:
	// get header
	acH, exists := c.Get("accessTarget")
	if !exists {
		log.Printf("access target header was not set properly")
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": "access target header was not set correctly"})

		return
	}
	ac := acH.(ut.AccessClaim)

	// get the resource info
	resource := &ut.Resource{
		Name:  ac.Target,
		Vname: ac.Vname,
	}
	var ar any = resource
	cancel, err := srv.storage.Download(&ar)
	if err != nil {
		log.Printf("error getting the download stream: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get download stream"})

		return
	}
	defer cancel()

	if resource.Reader == nil {
		log.Printf("resource reader is nil")
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})

		return
	}

	// parse byte range header
	var start, end, totalLength int64
	start, end, totalLength = 0, 4095, resource.Size

	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		// Expected format: "bytes=0-1023"
		parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
		if len(parts) == 2 {
			if s, err := strconv.Atoi(parts[0]); err == nil {
				start = int64(s)
			}
			if e, err := strconv.Atoi(parts[1]); err == nil {
				end = int64(e)
			}
		}
	}
	if start > totalLength {
		c.JSON(http.StatusRequestedRangeNotSatisfiable,
			gin.H{"error": "Requested range exceeds file size"})

		return
	}
	if end >= totalLength {
		end = totalLength - 1
	}

	length := end - start + 1
	pContent := make([]byte, length)

	readerAt, ok := resource.Reader.(io.ReaderAt)
	if !ok {
		// Wrap it if it's a byte slice or similar
		log.Printf("failed to cast as readerat, buffering reader...")
		var content []byte
		_, err = resource.Reader.Read(content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read resoruce"})
		}
		readerAt = bytes.NewReader(content) // if you have raw []byte
	}

	_, err = readerAt.ReadAt(pContent, start)
	if err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read resource"})

		return
	}
	c.Header("Content-Range", "bytes "+strconv.FormatInt(start, 10)+
		"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(totalLength, 10))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", strconv.Itoa(len(pContent)))
	c.Data(http.StatusPartialContent, "text/plain", pContent)
}

func buildTreeRec(tree map[string]any, entry []string, resource ut.Resource) {
	// Check if the current level already exists in the tree
	if len(entry) == 1 {
		tree[entry[0]] = resource

		return
	} else if _, exists := tree[entry[0]]; !exists {
		tree[entry[0]] = make(map[string]any)
	}

	buildTreeRec(tree[entry[0]].(map[string]any), entry[1:], resource)
}

// chmodResourceHandler updates the permission string of a resource.
//
// @Summary Change resource permissions
// @Description Updates permissions for a resource given its ID and a permission string.
// @Tags resources, admin
//
// @Accept application/x-www-form-urlencoded
// @Produce json
//
// @Param rid query string true "Resource ID"
// @Param permissions formData string true "New permission string (e.g., rwxr-x---)"
//
// @Success 200 {object} map[string]string "Resource updated successfully"
// @Failure 400 {object} map[string]string "Missing or invalid parameters"
// @Failure 500 {object} map[string]string "Failed to update resource"
//
// @Router /resource/permissions [post]
func (srv *UService) chmodResourceHandler(c *gin.Context) {
	rid := c.Request.URL.Query().Get("rid")
	if rid == "" {
		log.Printf("empty rid, not allowed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a rid"})

		return
	}
	newPerms := c.PostForm("permissions")
	if newPerms == "" {
		log.Printf("empty perms, not allowed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide perms as formvalue"})

		return
	}

	// update resource name
	err := srv.fsl.Update(map[string]string{"rid": rid, "perms": newPerms})
	if err != nil {
		log.Printf("error updating resource perms: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})

		return
	}

	c.JSON(200, gin.H{
		"message": "resource updated successfully",
	})
}

// chownResourceHandler changes the owner of a resource.
//
// @Summary Change resource owner
// @Description Updates the owner (user ID) of a resource based on its resource ID.
// @Tags resources, admin
//
// @Accept application/x-www-form-urlencoded
// @Produce json
//
// @Param rid query string true "Resource ID"
// @Param owner formData string true "New owner user ID"
//
// @Success 200 {object} map[string]string "Resource owner updated successfully"
// @Failure 400 {object} map[string]string "Missing or invalid parameters"
// @Failure 500 {object} map[string]string "Failed to update resource"
//
// @Router /resource/ownership [post]
func (srv *UService) chownResourceHandler(c *gin.Context) {
	rid := c.Request.URL.Query().Get("rid")
	if rid == "" {
		log.Printf("empty rid, not allowed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a rid"})

		return
	}
	newOwner := c.PostForm("owner")
	log.Printf("owner id: %v", newOwner)
	if newOwner == "" {
		log.Printf("empty uid, not allowed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a uid as formvalue"})

		return
	}

	// update resource name
	err := srv.fsl.Update(map[string]string{"rid": rid, "owner": newOwner})
	if err != nil {
		log.Printf("error updating resource uid: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})

		return
	}

	c.JSON(200, gin.H{
		"message": "resource updated successfully",
	})
}

// chgroupResourceHandler changes the group of a resource.
//
// @Summary Change resource group
// @Description Updates the group (group ID) of a resource based on its resource ID.
// @Tags resources, admin
//
// @Accept application/x-www-form-urlencoded
// @Produce json
//
// @Param rid query string true "Resource ID"
// @Param group formData string true "New group ID"
//
// @Success 200 {object} map[string]string "Resource group updated successfully"
// @Failure 400 {object} map[string]string "Missing or invalid parameters"
// @Failure 500 {object} map[string]string "Failed to update resource"
//
// @Router /resource/group [post]
func (srv *UService) chgroupResourceHandler(c *gin.Context) {
	rid := c.Request.URL.Query().Get("rid")
	if rid == "" {
		log.Printf("empty rid, not allowed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a rid"})

		return
	}
	newGroup := c.PostForm("group")
	if newGroup == "" {
		log.Printf("empty gid, not allowed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a gid as formvalue"})

		return
	}

	// update resource name
	err := srv.fsl.Update(map[string]string{"rid": rid, "group": newGroup})
	if err != nil {
		log.Printf("error updating resource group: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})

		return
	}

	c.JSON(200, gin.H{
		"message": "resource updated successfully",
	})
}
