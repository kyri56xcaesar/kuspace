package userspace

/*
	http api handlers for the userspace service
	"resource" related endpoints
*/

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	ut "kyri56xcaesar/myThesis/internal/utils"
)

/* perhaps move here all the  Resource specific funcs/methods/structs/models...*/
// @SEE models.go

/* HTTP Gin handlers related to resources, used by the api to handle endpoints*/

/*
* this should behave as:
* 'ls'
* */
func (srv *UService) getResourcesHandler(c *gin.Context) {
	l := c.Request.URL.Query().Get("limit")
	limit, err := strconv.Atoi(l)
	if err != nil {
		limit = 0
	}

	// get header
	ac_h, exists := c.Get("accessTarget")
	if !exists {
		log.Printf("access target header was not set properly")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "access target header was not set correctly"})
		return
	}
	ac := ac_h.(ut.AccessClaim)

	resources, err := srv.storage.SelectObjects(
		map[string]any{
			"vname":  ac.Vname,
			"prefix": ac.Target,
			"limit":  limit,
		},
	)

	// log.Printf("ac: %+v\nresources: %+v", ac, resources)
	if err != nil {
		log.Printf("error retrieving resource: %v", err)
		if strings.Contains(err.Error(), "scan") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "fatal"})
		}
		return
	} else if resources == nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "no objects found"})
		return
	}

	// as?

	// we should determine the structure to be returned.
	// this is given as uri argument
	// default is list
	structure := c.Request.URL.Query().Get("struct")

	switch structure {
	case "list":
		c.JSON(200, resources)
	case "tree":
		// build the tree and return it in json
		// need to parse all the resources
		tree := make(map[string]any)
		for _, res := range resources {
			resource := res.(ut.Resource)
			// buildTreeRec(tree, strings.Split(strings.TrimPrefix(resource.Name, "/"), "/"), resource)
			buildTreeRec(tree, append([]string{"/"}, strings.Split(strings.TrimPrefix(resource.Name, "/"), "/")...), resource)
		}

		c.JSON(200, tree)
	case "content":
		c.JSON(200, gin.H{"content": resources})
	default:
		c.JSON(200, resources)
	}
}

func (srv *UService) rmResourceHandler(c *gin.Context) {
	// get header
	ac_h, exists := c.Get("accessTarget")
	if !exists {
		log.Printf("access target header was not set properly")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "access target header was not set correctly"})
		return
	}
	ac := ac_h.(ut.AccessClaim)

	log.Printf("binded access claim: %+v", ac)

	target := c.Request.URL.Query().Get("rids")
	if target == "" {
		log.Printf("must provide a target")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a target"})
		return
	}
	// rids_str := strings.Split(target, ",")

	// needs to return some info bout what is deleted, lets do the size
	// size, err := srv.storage.Remove(rids_str)
	// if err != nil {
	// 	log.Printf("failed to delete resource: %v", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete resource"})
	// 	return
	// }

	// // release the volume space

	// err = srv.ReleaseVolumeSpace(size, ac)
	// if err != nil {
	// 	log.Printf("failed to release volume space: %v", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to release volume space"})
	// 	return
	// }

	// delete the phyiscal data (on the volume)
	// @TODO:
	//

	c.JSON(200, gin.H{
		"message": "resource deleted successfully.",
	})
}

func (srv *UService) mvResourcesHandler(c *gin.Context) {
	rid := c.Request.URL.Query().Get("rid")
	if rid == "" {
		log.Printf("empty rid, not allowed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a rid"})
		return
	}
	newName := c.Request.FormValue("resourcename")
	if newName == "" {
		log.Printf("empty resourcename, not allowed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a resourcename as formvalue"})
		return
	}

	// update resource name
	// err := srv.storage.Update(rid, newName)
	// if err != nil {
	// 	log.Printf("error updating resource name: %v", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})
	// 	return
	// }

	c.JSON(200, gin.H{
		"message": "resource updated successfully",
	})
}

func (srv *UService) cpResourceHandler(c *gin.Context) {
	c.JSON(200, gin.H{"message": "tbd"})
}

func (srv *UService) chmodResourceHandler(c *gin.Context) {
	rid := c.Request.URL.Query().Get("rid")
	if rid == "" {
		log.Printf("empty rid, not allowed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a rid"})
		return
	}
	newPerms := c.PostForm("permissions")
	log.Printf("perms: %v", newPerms)
	if newPerms == "" {
		log.Printf("empty perms, not allowed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide perms as formvalue"})
		return
	}

	// update resource name
	// err := srv.storage.Update(rid, newPerms)
	// if err != nil {
	// 	log.Printf("error updating resource perms: %v", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})
	// 	return
	// }

	c.JSON(200, gin.H{
		"message": "resource updated successfully",
	})
}

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

	// rid_int, err := strconv.Atoi(rid)
	// if err != nil {
	// 	log.Printf("failed to atoi rid: %v", err)
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": "bad request format"})
	// 	return
	// }
	// newOwner_int, err := strconv.Atoi(newOwner)
	// if err != nil {
	// 	log.Printf("failed to atoi ids: %v", err)
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": "bad request format"})
	// }
	// update resource name
	// err = srv.storage.Update(rid_int, newOwner_int)
	// if err != nil {
	// 	log.Printf("error updating resource uid: %v", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})
	// 	return
	// }

	c.JSON(200, gin.H{
		"message": "resource updated successfully",
	})
}

func (srv *UService) chgroupResourceHandler(c *gin.Context) {
	rid := c.Request.URL.Query().Get("rid")
	if rid == "" {
		log.Printf("empty rid, not allowed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a rid"})
		return
	}
	newGroup := c.PostForm("group")
	log.Printf("new gid: %v", newGroup)
	if newGroup == "" {
		log.Printf("empty gid, not allowed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a gid as formvalue"})
		return
	}

	// update resource name
	// rid_int, err := strconv.Atoi(rid)
	// if err != nil {
	// 	log.Printf("failed to atoi rid: %v", err)
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": "bad request format"})
	// 	return
	// }
	// newGroup_int, err := strconv.Atoi(newGroup)
	// if err != nil {
	// 	log.Printf("failed to atoi ids: %v", err)
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": "bad request format"})
	// }
	// err = srv.storage.Update(rid_int, newGroup_int)
	// if err != nil {
	// 	log.Printf("error updating resource group: %v", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})
	// 	return
	// }

	c.JSON(200, gin.H{
		"message": "resource updated successfully",
	})
}

func (srv *UService) handleDownload(c *gin.Context) {
	/* 1]: parse location from header*/
	// get header
	ac_h, exists := c.Get("accessTarget")
	if !exists {
		log.Printf("access target header was not set properly")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "access target header was not set correctly"})
		return
	}
	ac := ac_h.(ut.AccessClaim)
	log.Printf("binded access claim: %+v", ac)

	if strings.HasSuffix(ac.Target, "/") {
		log.Printf("target should be a file, not a directory")
		c.JSON(http.StatusBadRequest, gin.H{"error": "target should be a file, not a directory"})
		return
	}
	// get the path
	parts := strings.Split(ac.Target, "/")
	path := strings.Join(parts[1:], "/")
	name := parts[len(parts)-1]
	vid, err := strconv.Atoi(ac.Vid)
	if err != nil && ac.Vname == "" {
		log.Printf("failed to atoi vid: %v and vname not provided", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad vid"})
		return
	}

	resource := &ut.Resource{
		Path:  path,
		Name:  name,
		Vname: ac.Vname,
		Vid:   vid,
		Size:  -1,
	}
	var a_r any = resource

	cancelFn, err := srv.storage.Download(&a_r)
	defer cancelFn()
	if err != nil {
		log.Printf("failed to retrieve resource: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to retrieve resource"})
		return
	}

	if resource.Reader == nil {
		log.Printf("resource reader is nil")
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		return
	}

	c.DataFromReader(http.StatusOK, resource.Size, resource.Name, resource.Reader, map[string]string{
		"Content-Disposition": "attachment; filename=\"" + resource.Name + "\"",
		"Content-Type":        "application/octet-stream",
	})

}

/* the main endpoint handler for resource uploading */
func (srv *UService) handleUpload(c *gin.Context) {
	/* 1]: parse location from header*/
	// get header
	ac_h, exists := c.Get("accessTarget")
	if !exists {
		log.Printf("access target header was not set properly")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "access target header was not set correctly"})
		return
	}
	ac, ok := ac_h.(ut.AccessClaim)
	if !ok {
		log.Printf("failed to cast, ac wasn't set properly")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "access target wasn't set properly"})
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
		uid, err := strconv.Atoi(ac.Uid)
		if err != nil {
			log.Printf("failed to atoi uid: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad uid"})
			return
		}
		vid, err := strconv.Atoi(ac.Vid)
		if err != nil {
			log.Printf("failed to atoi vid: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad vid"})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("failed to read uploaded file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "fatal, failed to read uploaded files"})
			return
		}
		defer file.Close()

		currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
		/* Insert the appropriate metadata as a resource */
		resource := ut.Resource{
			Vid:    vid,
			Vname:  ac.Vname,
			Name:   ac.Target + fileHeader.Filename,
			Path:   ac.Target,
			Type:   "file",
			Reader: file,

			Created_at:  currentTime,
			Updated_at:  currentTime,
			Accessed_at: currentTime,
			Perms:       "rw-r--r--",
			Uid:         uid,
			Gid:         uid,
			Size:        int64(fileHeader.Size),
		}

		cancelFn, err := srv.storage.Insert(resource)
		defer cancelFn()
		if err != nil {
			log.Printf("failed to insert resources: %v", err)
			c.JSON(422, gin.H{"error": "failed to insert resources"})
			return
		}
	}
	c.JSON(200, gin.H{
		"message": "file/s uploaded.",
	})
}

// func (srv *UService) handlePreview(c *gin.Context) {
// 	// parse resource target header:
// 	// get header
// 	ac_h, exists := c.Get("accessTarget")
// 	if !exists {
// 		log.Printf("access target header was not set properly")
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "access target header was not set correctly"})
// 		return
// 	}
// 	ac := ac_h.(ut.AccessClaim)

// 	path := strings.TrimSuffix(ac.Target, "/")
// 	// get the resource info
// 	res, err := srv.storage.SelectOne("", "resources", "name = ", path)
// 	if err != nil {
// 		log.Printf("failed to get the resource: %v", err)
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "resource not found"})
// 		return
// 	}
// 	resource := res.(ut.Resource)
// 	// read the actual file to a buffer

// 	// parse byte range header
// 	var start, end, totalLength int64
// 	start, end, totalLength = 0, 4095, resource.Size
// 	rangeHeader := c.GetHeader("Range")
// 	if rangeHeader != "" {
// 		// Expected format: "bytes=0-1023"
// 		parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
// 		if len(parts) == 2 {
// 			if s, err := strconv.Atoi(parts[0]); err == nil {
// 				start = int64(s)
// 			}
// 			if e, err := strconv.Atoi(parts[1]); err == nil {
// 				end = int64(e)
// 			}
// 		}
// 	}
// 	if start > totalLength {
// 		c.JSON(http.StatusRequestedRangeNotSatisfiable, gin.H{"error": "Requested range exceeds file size"})
// 		return
// 	}
// 	if end >= totalLength {
// 		end = totalLength - 1
// 	}

// 	pContent, err := fetchResource(srv.config.VOLUMES_PATH+path, int64(start), int64(end))
// 	if err != nil {
// 		log.Printf("failed to fetch resource: %v", err)
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch resoruc"})
// 		return
// 	}

// 	c.Header("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(totalLength, 10))
// 	c.Header("Accept-Ranges", "bytes")
// 	c.Header("Content-Length", strconv.Itoa(len(pContent)))
// 	c.Data(http.StatusPartialContent, "text/plain", pContent)
// }

// fetchResource reads a file from the given path within the specified byte range.
func fetchResource(filePath string, start, end int64) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	if start >= fileSize {
		return nil, errors.New("requested range exceeds file size")
	}
	if end >= fileSize {
		end = fileSize - 1
	}

	_, err = file.Seek(start, io.SeekStart)
	if err != nil {
		return nil, err
	}

	data := make([]byte, end-start+1)
	_, err = file.Read(data)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return data, nil
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
