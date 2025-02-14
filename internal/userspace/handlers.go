package userspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	ut "kyri56xcaesar/myThesis/internal/utils"

	"github.com/gin-gonic/gin"
)

/* A custom HTTP header parser
*
* AccessTarget=<filepath> <signature>
*
* <filepath>: the name of the file you wish to acccess
* (if it ends in /, it means its a directory)
*
* <signature>:your_user_id:[group_id,groupd_id,...]
* so the signature plainly is the user ID delimitted by ':'
* followed by the group ids (delimitted by commas).
*
*
* */
func BindAccessTarget(http_header string) (AccessClaim, error) {
	parts := strings.SplitN(http_header, " ", 2)
	if len(parts) != 2 {
		return AccessClaim{}, fmt.Errorf("invalid header format")
	}

	target := parts[0]
	if !strings.HasSuffix(target, "/") {
		target += "/"
	}

	sig := parts[1]
	p := strings.SplitN(sig, ":", 2)
	if len(p) != 2 {
		return AccessClaim{}, fmt.Errorf("invalid signature format")
	}
	if p == nil || p[0] == "" || p[1] == "" {
		return AccessClaim{}, fmt.Errorf("nil parameters")
	}

	return AccessClaim{
		Uid:    p[0],
		Gids:   p[1],
		Target: target,
	}, nil
}

/* 'resource' handlers
* SEE: models.go
*
* WARNING: Deprecated
* */
func (srv *UService) GetResourceHandler(c *gin.Context) {
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil {
		log.Printf("failed to bind access-target: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
		return
	}
	log.Printf("binded access claim: %+v", ac)
	/* check if claim is valid */
	/* It is checked on binding rn
	  if err := ac.validate(); err != nil {
			log.Printf("claim not valid: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	*/

	/* find target resource */
	resource, err := srv.dbh.GetResourceByFilepath(ac.Target)
	if err != nil {
		log.Printf("error retrieving resource: %v", err)
		if strings.Contains(err.Error(), "scan") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "fatal"})
		}
		return
	}

	/* Check for access authorization */
	/* This method requires Read Access to the Resource */
	if !resource.HasAccess(ac) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	c.JSON(200, resource)
}

/*
* this should behave as:
* 'ls'
* */
func (srv *UService) ResourcesHandler(c *gin.Context) {
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil {
		log.Printf("failed to bind access-target: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
		return
	}
	resources, err := srv.dbh.GetAllResourcesAt(ac.Target + "%")
	if err != nil {
		log.Printf("error retrieving resource: %v", err)
		if strings.Contains(err.Error(), "scan") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "fatal"})
		}
		return
	} else if resources == nil {
	}

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
		tree := make(map[string]interface{})
		for _, resource := range resources {
			// buildTreeRec(tree, strings.Split(strings.TrimPrefix(resource.Name, "/"), "/"), resource)
			buildTreeRec(tree, append([]string{"/"}, strings.Split(strings.TrimPrefix(resource.Name, "/"), "/")...), resource)
		}

		c.JSON(200, tree)
	default:
		c.JSON(200, resources)
	}
}

func buildTreeRec(tree map[string]interface{}, entry []string, resource Resource) {
	// Check if the current level already exists in the tree
	if len(entry) == 1 {
		tree[entry[0]] = resource
		return
	} else if _, exists := tree[entry[0]]; !exists {
		tree[entry[0]] = make(map[string]interface{})
	}

	buildTreeRec(tree[entry[0]].(map[string]interface{}), entry[1:], resource)
}

/* this should behave as:
* 'mkdir' for directory types,
* for file types it should trigger file upload
* simple resource
*
* WARNING: don't use this...
* resource uploading is sufficient, "directories" are pseudo elements.
* resource path is enough.
* */
func (srv *UService) PostResourcesHandler(c *gin.Context) {
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil {
		log.Printf("failed to bind access-target: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
		return
	}
	log.Printf("binded access claim: %+v", ac)

	var resources []Resource
	err = c.BindJSON(&resources)
	if err != nil {
		log.Printf("error binding: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad binding"})
		return
	}
	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
	for i := range resources {
		resources[i].Name = ac.Target + resources[i].Name
		resources[i].Created_at = currentTime
		resources[i].Updated_at = currentTime
		resources[i].Accessed_at = currentTime
	}

	log.Printf("binded resources: %+v", resources)

	err = srv.dbh.InsertResources(resources)
	if err != nil {
		log.Printf("failed to insert resources: %v", err)
		c.JSON(422, gin.H{"error": "failed to insert resources"})
		return
	}

	c.JSON(200, gin.H{
		"message": "resources inserted",
	})
}

func (srv *UService) RemoveResourceHandler(c *gin.Context) {
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil {
		log.Printf("failed to bind access-target: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
		return
	}
	ac.Vid = 1 // for now
	log.Printf("binded access claim: %+v", ac)

	target := c.Request.URL.Query().Get("rids")
	if target == "" {
		log.Printf("must provide a target")
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a target"})
		return
	}
	rids_str := strings.Split(target, ",")

	// needs to return some info bout what is deleted, lets do the size
	size, err := srv.dbh.DeleteResourcesByIds(rids_str)
	if err != nil {
		log.Printf("failed to delete resource: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete resource"})
		return
	}

	// release the volume space

	err = srv.ReleaseVolumeSpace(size, ac)

	// delete the phyiscal data (on the volume)
	// @TODO:
	//

	c.JSON(200, gin.H{
		"message": "resource deleted successfully.",
	})
}

func (srv *UService) MoveResourcesHandler(c *gin.Context) {
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
	err := srv.dbh.UpdateResourceNameById(rid, newName)
	if err != nil {
		log.Printf("error updating resource name: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})
		return
	}

	c.JSON(200, gin.H{
		"message": "resource updated successfully",
	})
}

func (srv *UService) ChmodResourceHandler(c *gin.Context) {
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
	err := srv.dbh.UpdateResourcePermsById(rid, newPerms)
	if err != nil {
		log.Printf("error updating resource perms: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})
		return
	}

	c.JSON(200, gin.H{
		"message": "resource updated successfully",
	})
}

func (srv *UService) ChownResourceHandler(c *gin.Context) {
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

	rid_int, err := strconv.Atoi(rid)
	newOwner_int, err := strconv.Atoi(newOwner)
	if err != nil {
		log.Printf("failed to atoi ids: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request format"})
	}
	// update resource name
	err = srv.dbh.UpdateResourceOwnerById(rid_int, newOwner_int)
	if err != nil {
		log.Printf("error updating resource uid: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})
		return
	}

	c.JSON(200, gin.H{
		"message": "resource updated successfully",
	})
}

func (srv *UService) ChgroupResourceHandler(c *gin.Context) {
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
	rid_int, err := strconv.Atoi(rid)
	newGroup_int, err := strconv.Atoi(newGroup)
	if err != nil {
		log.Printf("failed to atoi ids: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request format"})
	}
	err = srv.dbh.UpdateResourceGroupById(rid_int, newGroup_int)
	if err != nil {
		log.Printf("error updating resource group: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})
		return
	}

	c.JSON(200, gin.H{
		"message": "resource updated successfully",
	})
}

func (srv *UService) ResourceCpHandler(c *gin.Context) {
	c.JSON(200, gin.H{"message": "tbd"})
}

func (srv *UService) HandleDownload(c *gin.Context) {
	/* 1]: parse location from header*/
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil {
		log.Printf("failed to bind access-target: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
		return
	}
	log.Printf("binded access claim: %+v", ac)
	log.Printf("trimmed: %v", strings.TrimSuffix(ac.Target, "/"))
	path := strings.TrimSuffix(ac.Target, "/")
	_, err = srv.dbh.GetResourceByFilepath(path)
	if err != nil {
		log.Printf("failed to retrieve resource: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to retrieve resource"})
		return
	}

	split := strings.Split(path, "/")
	c.FileAttachment(srv.config.Volumes+path, split[len(split)-1])
}

/* the main endpoint handler for resource uploading */
func (srv *UService) HandleUpload(c *gin.Context) {
	/* 1]: parse location from header*/
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	log.Printf("ac: %+v", ac)
	if err != nil {
		log.Printf("failed to bind access-target: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
		return
	}
	// 2]: we should check if destination is valid and if user is authorizated
	/*
	* */
	if !strings.HasPrefix(ac.Target, "/") {
		log.Printf("invalid target path")
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad target path, must provide a directory"})
		return
	}

	// 3]: determine physical destination path
	// parse the form files
	err = c.Request.ParseMultipartForm(10 << 10)
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

	// 3.1] Should check if user is limited by a quota
	err = srv.ClaimVolumeSpace(totalUploadSize, ac)
	if err != nil {
		log.Printf("unable to proceed with resource: %v", err)
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	// perhaphs we could avoid this step, since we checking frm volume metadata
	// 3.2] or by the system..
	physicalPath, err := determinePhysicalStorage(srv.config.Volumes+ac.Target, totalUploadSize)
	if err != nil {
		log.Printf("could't establish physical storage: %v", err)
		c.JSON(http.StatusInsufficientStorage, gin.H{"error": "storage failure"})
		return
	}

	// 4]: perform the upload stream
	/* I would like to do this concurrently perpahps*/
	for _, fileHeader := range c.Request.MultipartForm.File["files"] {
		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("failed to read uploaded file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "fatal, failed to read uploaded files"})
			return
		}
		defer file.Close()

		outFile, err := os.Create(physicalPath + fileHeader.Filename)
		if err != nil {
			log.Printf("failed to create output file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create output file"})
			return
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, file)
		if err != nil {
			log.Printf("failed to save file to storage: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
			return
		}

		uid, err := strconv.Atoi(ac.Uid)
		if err != nil {
			log.Printf("failed to atoi uid: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad uid"})
			return
		}
		currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
		/* Insert the appropriate metadata as a resource */
		//currentTime := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
		resource := Resource{
			Name:        ac.Target + fileHeader.Filename,
			Type:        "file",
			Created_at:  currentTime,
			Updated_at:  currentTime,
			Accessed_at: currentTime,
			Perms:       "rw-r--r--",
			Uid:         uid,
			Gid:         uid,
			Size:        int(fileHeader.Size),
		}

		err = srv.dbh.InsertResourceUniqueName(resource)
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

func (srv *UService) HandlePreview(c *gin.Context) {
	// parse resource target header:
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil {
		log.Printf("failed to bind access-target: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Access-Target header"})
		return
	}
	log.Printf("binded access claim: %+v", ac)
	path := strings.TrimSuffix(ac.Target, "/")
	// get the resource info
	resource, err := srv.dbh.GetResourceByFilepath(path)
	if err != nil {
		log.Printf("failed to get the resource: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource not found"})
		return
	}

	// read the actual file to a buffer

	// parse byte range header
	start, end, totalLength := 0, 4095, resource.Size
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		// Expected format: "bytes=0-1023"
		parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
		if len(parts) == 2 {
			if s, err := strconv.Atoi(parts[0]); err == nil {
				start = s
			}
			if e, err := strconv.Atoi(parts[1]); err == nil {
				end = e
			}
		}
	}
	if start > totalLength {
		c.JSON(http.StatusRequestedRangeNotSatisfiable, gin.H{"error": "Requested range exceeds file size"})
		return
	}
	if end >= totalLength {
		end = totalLength - 1
	}

	pContent, err := fetchResource(srv.config.Volumes+path, int64(start), int64(end))
	if err != nil {
		log.Printf("failed to fetch resource: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch resoruc"})
		return
	}

	c.Header("Content-Range", "bytes "+strconv.Itoa(start)+"-"+strconv.Itoa(end)+"/"+strconv.Itoa(totalLength))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", strconv.Itoa(len(pContent)))
	c.Data(http.StatusPartialContent, "text/plain", pContent)
}

func (srv *UService) HandleVolumes(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:

		vid := c.Request.URL.Query().Get("vid")
		if vid != "" {
			vid_int, err := strconv.Atoi(vid)
			if err != nil {
				log.Printf("failed to atoi vid: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad vid"})
				return
			}
			volume, err := srv.dbh.GetVolumeByVid(vid_int)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"content": volume})
		} else {
			volumes, err := srv.dbh.GetVolumes()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"content": volumes})
		}

	case http.MethodPut:
		c.JSON(200, gin.H{"status": "tbd"})
	case http.MethodDelete:
		vid := c.Request.URL.Query().Get("vid")
		if vid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "must provide vid"})
			return
		}
		vid_int, err := strconv.Atoi(vid)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "couldn't convert to int"})
			return
		}

		err = srv.dbh.DeleteVolume(vid_int)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete the volume"})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{"status": "successfully deleted volume"})

	case http.MethodPatch:
		c.JSON(200, gin.H{"status": "tbd"})
	case http.MethodPost:
		var volumes []Volume
		err := c.BindJSON(&volumes)
		if err != nil {
			log.Printf("failed to bind volumes: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "couldn't bind volumes"})
			return
		}
		err = srv.dbh.InsertVolumes(volumes)
		if err != nil {
			log.Printf("failed to insert volumes: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "couldn't insert volumes"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"error": "inserted volume(s)"})
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "not allowed."})
	}
}

func (srv *UService) HandleUserVolumes(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodPost:
		var (
			userVolumes []UserVolume
			userVolume  UserVolume
		)

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("failed to read request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}

		err = json.Unmarshal(body, &userVolumes)
		if err != nil {
			err = json.Unmarshal(body, &userVolume)
			// single userVolume
			if err != nil {
				log.Printf("fail to bind body: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad request, failed to bind"})
				return
			}
			err = srv.dbh.InsertUserVolume(userVolume)
			if err != nil {
				log.Printf("failed to insert user volume: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert uv"})
				return
			}
			c.JSON(http.StatusCreated, gin.H{"status": "inserted user volume"})
			return
		}
		// binded user
		err = srv.dbh.InsertUserVolumes(userVolumes)
		if err != nil {
			log.Printf("failed to insert user volumes: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert uv"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"status": "inserted user volumes"})

	case http.MethodDelete:
	case http.MethodPatch:

	case http.MethodGet:
		uids := c.Request.URL.Query().Get("uids")
		vids := c.Request.URL.Query().Get("vids")
		var (
			userVolumes []UserVolume
			data        interface{}
			err         error
		)

		if uids == "" && vids == "" {
			// return all
			data, err = srv.dbh.GetUserVolumes()
			if err != nil {
				log.Printf("failed to retrieve user volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user volumes"})
				return
			}
			userVolumes = data.([]UserVolume)
		} else if uids == "" {
			// return by vids
			data, err = srv.dbh.GetUserVolumesByVolumeIds(strings.Split(strings.TrimSpace(vids), ","))
			if err != nil {
				log.Printf("failed to retrieve user volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user volumes"})
				return
			}
			userVolumes = data.([]UserVolume)
		} else if vids == "" {
			// return by uids
			data, err = srv.dbh.GetUserVolumesByUserIds(strings.Split(strings.TrimSpace(uids), ","))
			if err != nil {
				log.Printf("failed to retrieve user volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user volumes"})
				return
			}
			userVolumes = data.([]UserVolume)
		} else {
			// return by both
			data, err = srv.dbh.GetUserVolumesByUidsAndVids(strings.Split(strings.TrimSpace(uids), ","), strings.Split(strings.TrimSpace(vids), ","))
			if err != nil {
				log.Printf("failed to retrieve user volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user volumes"})
				return
			}
			userVolumes = data.([]UserVolume)
		}
		c.JSON(http.StatusOK, gin.H{"content": userVolumes})

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

func (srv *UService) HandleGroupVolumes(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodPost:
		var (
			groupVolumes []GroupVolume
			groupVolume  GroupVolume
		)

		err := c.BindJSON(&groupVolumes)
		if err != nil {
			log.Printf("didn't bind groupVolumes, lets try a groupVolume..")
			err = c.BindJSON(&groupVolume)
			// single userVolume
			if err != nil {
				log.Printf("fail to bind body: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad request, failed to bind"})
				return
			}
			err = srv.dbh.InsertGroupVolume(groupVolume)
			if err != nil {
				log.Printf("failed to insert group volume: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert gv"})
				return
			}
			c.JSON(http.StatusCreated, gin.H{"status": "inserted group volume"})
			return
		}
		// binded user
		err = srv.dbh.InsertGroupVolumes(groupVolumes)
		if err != nil {
			log.Printf("failed to insert group volumes: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert gv"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"status": "inserted group volumes"})

	case http.MethodDelete:
	case http.MethodPatch:
	case http.MethodGet:
		gids := c.Request.URL.Query().Get("gids")
		vids := c.Request.URL.Query().Get("vids")
		var (
			groupVolumes []GroupVolume
			data         interface{}
			err          error
		)

		if gids == "" && vids == "" {
			// return all
			data, err = srv.dbh.GetGroupVolumes()
			if err != nil {
				log.Printf("failed to retrieve group volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve group volumes"})
				return
			}
			groupVolumes = data.([]GroupVolume)
		} else if gids == "" {
			// return by vids
			data, err = srv.dbh.GetGroupVolumesByVolumeIds(strings.Split(strings.TrimSpace(vids), ","))
			if err != nil {
				log.Printf("failed to retrieve group volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve group volumes"})
				return
			}
			groupVolumes = data.([]GroupVolume)
		} else if vids == "" {
			// return by uids
			data, err = srv.dbh.GetGroupVolumesByGroupIds(strings.Split(strings.TrimSpace(gids), ","))
			if err != nil {
				log.Printf("failed to retrieve group volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve group volumes"})
				return
			}
			groupVolumes = data.([]GroupVolume)
		} else {
			// return by both
			data, err = srv.dbh.GetGroupVolumesByVidsAndGids(strings.Split(strings.TrimSpace(vids), ","), strings.Split(strings.TrimSpace(gids), ","))
			if err != nil {
				log.Printf("failed to retrieve group volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retireve group volumes"})
				return
			}
			groupVolumes = data.([]GroupVolume)
		}
		c.JSON(http.StatusOK, gin.H{"content": groupVolumes})

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

func (srv *UService) ClaimVolumeSpace(size int64, ac AccessClaim) error {
	// for now:
	ac.Vid = 1
	uid, err := strconv.Atoi(ac.Uid)
	if err != nil {
		log.Printf("failed to atoi ac.Uid, shouldn't have passed till here tbh...:%v", err)
		return fmt.Errorf("atoi failure, shouldn't be here: %v", err)
	}
	volume, err := srv.dbh.GetVolumeByVid(ac.Vid)
	if err != nil {
		log.Printf("could not retrieve volume: %v", err)
		return fmt.Errorf("could not retrieve volume: %w", err)
	}
	// check for current volume usage.
	// size is in Bytes
	size_inGB := float32(size) / 1000000000
	new_usage_inGB := volume.Usage + size_inGB

	if new_usage_inGB > volume.Capacity {
		log.Printf("volume is full.")
		return fmt.Errorf("claim exceeds capacity")
	}

	// if not dynamic, we should check for per user/group quota
	uv, err := srv.dbh.GetUserVolumeByUid(uid)
	if err != nil {
		log.Printf("failed to retrieve user volume: %v", err)
		return err
	}
	gid, err := strconv.Atoi(strings.Split(strings.TrimSpace(ac.Gids), ",")[0])
	if err != nil {
		return fmt.Errorf("atoi failure, shouldn't be here: %v", err)
	}
	gv, err := srv.dbh.GetGroupVolumeByGid(gid)
	if err != nil {
		log.Printf("failed to retrieve group volume: %v", err)
		return err
	}

	// update all usages
	// volume
	// volume claims user/group
	uv.Usage += size_inGB
	gv.Usage += size_inGB
	volume.Usage = new_usage_inGB

	log.Printf("user volume: %+v", uv)
	log.Printf("group volume: %+v", gv)
	log.Printf("current size: %v, volume: %+v", size_inGB, volume)

	err = srv.dbh.UpdateVolume(volume)
	if err != nil {
		log.Printf("failed to update volume usages: %v", err)
		return err
	}
	err = srv.dbh.UpdateUserVolume(uv)
	if err != nil {
		log.Printf("failed to update user volume usages: %v", err)
		return err
	}
	err = srv.dbh.UpdateGroupVolume(gv)
	if err != nil {
		log.Printf("failed to update group volume usages: %v", err)
		return err
	}

	return nil
}

func (srv *UService) ReleaseVolumeSpace(size int64, ac AccessClaim) error {
	// for now:
	ac.Vid = 1
	uid, err := strconv.Atoi(ac.Uid)
	if err != nil {
		log.Printf("failed to atoi ac.Uid, shouldn't have passed till here tbh...:%v", err)
		return fmt.Errorf("atoi failure, shouldn't be here: %v", err)
	}
	volume, err := srv.dbh.GetVolumeByVid(ac.Vid)
	if err != nil {
		log.Printf("could not retrieve volume: %v", err)
		return fmt.Errorf("could not retrieve volume: %w", err)
	}

	size_inGB := float32(size) / 1000000000
	new_usage_inGB := volume.Usage - size_inGB

	if new_usage_inGB < 0 {
		new_usage_inGB = 0
	}

	uv, err := srv.dbh.GetUserVolumeByUid(uid)
	if err != nil {
		log.Printf("failed to retrieve user volume: %v", err)
		return err
	}
	gid, err := strconv.Atoi(strings.Split(strings.TrimSpace(ac.Gids), ",")[0])
	if err != nil {
		return fmt.Errorf("atoi failure, shouldn't be here: %v", err)
	}
	gv, err := srv.dbh.GetGroupVolumeByGid(gid)
	if err != nil {
		log.Printf("failed to retrieve group volume: %v", err)
		return err
	}

	// update all usages
	// volume
	// volume claims user/group
	uv.Usage -= size_inGB
	gv.Usage -= size_inGB
	volume.Usage = new_usage_inGB

	err = srv.dbh.UpdateVolume(volume)
	if err != nil {
		log.Printf("failed to update volume usages: %v", err)
		return err
	}
	err = srv.dbh.UpdateUserVolume(uv)
	if err != nil {
		log.Printf("failed to update user volume usages: %v", err)
		return err
	}
	err = srv.dbh.UpdateGroupVolume(gv)
	if err != nil {
		log.Printf("failed to update group volume usages: %v", err)
		return err
	}

	return nil
}

/* this should be determined by configurating Volume destination.
*  also it will ensure the destination location exists.
* */
func determinePhysicalStorage(target string, fileSize int64) (string, error) {
	targetParts := strings.Split(target, "/")
	availableSpace, err := ut.GetAvailableSpace(strings.Join(targetParts[:2], "/"))
	if err != nil {
		return "", fmt.Errorf("failed to get available space: %v", err)
	}

	if availableSpace < uint64(fileSize) {
		return "", fmt.Errorf("insufficient space")
	}

	_, err = os.Stat(targetParts[0])
	if err != nil {
		err = os.Mkdir(targetParts[0], 0o700)
		if err != nil {
			log.Printf("failed to mkdir: %v", err)
			return "", err
		}

		_, err = os.Stat(strings.Join(targetParts[:2], "/"))
		if err != nil {
			err = os.Mkdir(strings.Join(targetParts[:2], "/"), 0o700)
			if err != nil {
				log.Printf("failed to mkdir: %v", err)
				return "", err
			}
		}
	}

	for index, part := range targetParts[2:] {
		if part == "" || index == len(targetParts)-1 {
			continue
		}
		currPath := strings.Join(targetParts[:index], ",")
		_, err := os.Stat(currPath)
		if err != nil {
			err = os.Mkdir(currPath, 0o700)
			if err != nil {
				log.Printf("failed to mkdir: %v", err)
			}
		}
	}

	return target, nil
}

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
