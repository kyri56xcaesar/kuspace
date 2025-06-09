package uspace

/*
	http api handlers for the uspace service
	"volume" related endpoints
*/

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

// HandleVolumes manages volume resources (create, read, delete)
//
// @Summary     Manage volumes
// @Description GET to list volumes, POST to create one or more, DELETE to remove by vid
// @Tags        volumes
// @Accept      json
// @Produce     json
//
// @Param       vid    query     string        false  "Volume ID to filter (GET) or delete (DELETE)"
// @Param       limit  query     string        false  "Limit number of returned volumes"
// @Param       sort   query     string        false  "Sort order for volumes"
//
// @Param       volume body      ut.Volume     true   "Single volume object"
// @Param       volumes body     []ut.Volume   true   "Array of volume objects"
//
// @Success     200     {object}  map[string]interface{}  "Success with content"
// @Success     201     {object}  map[string]string       "Volume(s) created"
// @Success     202     {object}  map[string]string       "Volume deleted"
// @Failure     400     {object}  map[string]string       "Bad request or validation failure"
// @Failure     405     {object}  map[string]string       "Method not allowed"
// @Failure     500     {object}  map[string]string       "Internal server error"
//
// @Router      /volumes [get]
// @Router      /volumes [post]
// @Router      /volumes [delete]
// @Router      /volumes [patch]
// @Router      /volumes [put]
func (srv *UService) handleVolumes(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		vid := c.Request.URL.Query().Get("vid")

		names := []string{"vid", "limit", "sort"}
		values := []any{vid, c.Request.URL.Query().Get("limit"), c.Request.URL.Query().Get("sort")}

		volumes, err := srv.storage.SelectVolumes(ut.MakeMapFrom(names, values))
		if err != nil {
			log.Printf("[USPACE_API] failed to select volumes: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})

			return
		}
		c.JSON(http.StatusOK, gin.H{"content": volumes})
	case http.MethodDelete:
		vid := c.Query("volume")
		if vid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a volume name"})

			return
		}
		err := srv.storage.RemoveVolume(vid)
		if err != nil {
			log.Printf("[USPACE_API] failed to delete the volume: %v", err)
			if strings.Contains(err.Error(), "not empty") {
				c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete bucket if not empty"})

				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete the volume"})

			return
		}

		c.JSON(http.StatusAccepted, gin.H{"status": "successfully deleted volume"})
	case http.MethodPost:
		// read body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("[USPACE_API] failed to read request body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read req body"})

			return
		}

		// check for an array of volumes
		var volumes []ut.Volume
		err = json.Unmarshal(body, &volumes)
		if err != nil { // check for single volume
			var volume ut.Volume
			err = json.Unmarshal(body, &volume)
			if err != nil {
				log.Printf("[USPACE_API] failed to bind as a single volume as well, returning. Bad request: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})

				return
			}

			err = volume.Validate(srv.config.LocalVolumesDefaultCapacity, srv.config.LocalVolumesDefaultCapacity, "-.")
			if err != nil {
				log.Printf("[USPACE_API] failed to validate the volume info: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

				return
			}
			// single volume
			err = srv.storage.CreateVolume(any(volume))
			if err != nil {
				log.Printf("[USPACE_API] failed to insert volume: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "couldn't insert volume"})

				return
			}
		}
		// array of volumes
		// insert them iteratevly
		for _, volume := range volumes {
			err = volume.Validate(srv.config.LocalVolumesDefaultCapacity, srv.config.LocalVolumesDefaultCapacity, "-.")
			if err != nil {
				log.Printf("[USPACE_API] failed to validate the volume info: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

				return
			}
			err = srv.storage.CreateVolume(any(volume))
			if err != nil {
				log.Printf("[USPACE_API] failed to insert volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "couldn't insert volumes"})

				return
			}
		}

		c.JSON(http.StatusCreated, gin.H{"message": "inserted volume(s) successfully"})
	case http.MethodPatch:
		c.JSON(200, gin.H{"status": "tbd"})
	case http.MethodPut:
		c.JSON(200, gin.H{"status": "tbd"})
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "not allowed."})
	}
}

// handleUserVolumes manages user volume registration and updates.
//
// @Summary Manage user volumes
// @Description Insert single or multiple user volume objects.
// @Tags volumes, users
//
// @Accept json
// @Produce json
//
// @Param userVolume body ut.UserVolume true "Single user volume"
// @Param userVolumes body []ut.UserVolume true "Array of user volumes"
//
// @Success 201 {object} map[string]string "User volume(s) inserted"
// @Failure 400 {object} map[string]string "Bad request (binding or decoding error)"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 405 {object} map[string]string "Method not allowed"
//
// @Router /admin/user/volume [post]
// @Router /admin/user/volume [patch]
// @Router /admin/user/volume [delete]
func (srv *UService) handleUserVolumes(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodPost:
		var (
			userVolumes []ut.UserVolume
			userVolume  ut.UserVolume
		)

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("[USPACE_API] failed to read request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})

			return
		}

		err = json.Unmarshal(body, &userVolumes)
		if err != nil {
			err = json.Unmarshal(body, &userVolume)
			// single userVolume
			if err != nil {
				log.Printf("[USPACE_API] fail to bind body: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad request, failed to bind"})

				return
			}

			if capacity, _ := strconv.ParseFloat(srv.config.LocalVolumesDefaultPath, 64); int(userVolume.Quota) == 0 || userVolume.Quota > float64(capacity) {
				log.Printf("[USPACE_API] inserted")
				userVolume.Quota = float64(capacity)
			}

			cancelFn, err := srv.fsl.Insert([]any{userVolume})
			if err != nil {
				log.Printf("[USPACE_API] failed to insert user volume: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert uv"})

				return
			}
			defer cancelFn()
			c.JSON(http.StatusCreated, gin.H{"status": "inserted user volume"})

			return
		}
		// binded user
		cancelFn, err := srv.fsl.Insert([]any{userVolumes})
		defer cancelFn()
		if err != nil {
			log.Printf("[USPACE_API] failed to insert user volumes: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert uv"})

			return
		}
		c.JSON(http.StatusCreated, gin.H{"status": "inserted user volumes"})
	case http.MethodDelete:
	case http.MethodPatch:

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

func (srv *UService) handleGroupVolumes(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodPost:
		var (
			groupVolumes []ut.GroupVolume
			groupVolume  ut.GroupVolume
		)

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("[USPACE_API] failed to read request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})

			return
		}

		err = json.Unmarshal(body, &groupVolumes)
		if err != nil {
			log.Printf("[USPACE_API] didn't bind groupVolumes, lets try a groupVolume..")
			err = json.Unmarshal(body, &groupVolume)
			// single userVolume
			if err != nil {
				log.Printf("[USPACE_API] fail to bind body: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad request, failed to bind"})

				return
			}

			if capacity := min(srv.config.LocalVolumesDefaultCapacity, maxDefaultVolumeCapacity); int(groupVolume.Quota) == 0 || groupVolume.Quota > float64(capacity) {
				log.Printf("[USPACE_API] inserted")
				groupVolume.Quota = float64(capacity)
			}

			cancelFn, err := srv.storage.Insert([]any{groupVolume})
			defer cancelFn()
			if err != nil {
				log.Printf("[USPACE_API] failed to insert group volume: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert gv"})

				return
			}
			c.JSON(http.StatusCreated, gin.H{"status": "inserted group volume"})

			return
		}

		// binded user
		cancelFn, err := srv.storage.Insert([]any{groupVolumes})
		defer cancelFn()
		if err != nil {
			log.Printf("[USPACE_API] failed to insert group volumes: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert gv"})

			return
		}
		c.JSON(http.StatusCreated, gin.H{"status": "inserted group volumes"})

	case http.MethodDelete:
	case http.MethodPatch:

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

// these two funcs seem to be irrelevant here.. should belong to fslite
