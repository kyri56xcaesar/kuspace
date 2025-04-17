package userspace

/*
	http api handlers for the userspace service
	"volume" related endpoints
*/

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	ut "kyri56xcaesar/myThesis/internal/utils"

	"github.com/gin-gonic/gin"
)

/* HTTP Gin handlers related to volumes, uservolumes, groupvolumes, used by the api to handle endpoints*/

func (srv *UService) HandleVolumes(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		vid := c.Request.URL.Query().Get("vid")
		if vid != "" { // get a specific volume/bucket
			volume, err := srv.storage.SelectOne("", "volumes", "vid", vid)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"content": volume})
		} else { // get all volumes/buckets
			volumes, err := srv.storage.Select("", "volumes", "", "", 0)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
				return
			}
			log.Printf("volumes returned: %+v", volumes)
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
		err := srv.storage.Remove(vid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete the volume"})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{"status": "successfully deleted volume"})

	case http.MethodPatch:
		c.JSON(200, gin.H{"status": "tbd"})
	case http.MethodPost:
		// read body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("failed to read request body: %v", err)
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
				log.Printf("failed to bind as a single volume as well, returning. Bad request: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
				return
			}
			err = volume.Validate()
			if err != nil {
				log.Printf("failed to validate the volume info: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			// single volume
			err = srv.storage.CreateVolume(any(volume))
			if err != nil {
				log.Printf("failed to insert volume: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "couldn't insert volume"})
				return
			}
		}
		// array of volumes
		// insert them iteratevly
		for _, volume := range volumes {
			err = volume.Validate()
			if err != nil {
				log.Printf("failed to validate the volume info: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			err = srv.storage.CreateVolume(any(volume))
			if err != nil {
				log.Printf("failed to insert volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "couldn't insert volumes"})
				return
			}
		}

		c.JSON(http.StatusCreated, gin.H{"message": "inserted volume(s) successfully"})
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "not allowed."})
	}
}

func (srv *UService) HandleUserVolumes(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodPost:
		var (
			userVolumes []ut.UserVolume
			userVolume  ut.UserVolume
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

			if capacity, _ := strconv.ParseFloat(srv.config.V_DEFAULT_CAPACITY, 64); int(userVolume.Quota) == 0 || userVolume.Quota > float64(capacity) {
				log.Printf("inserted")
				userVolume.Quota = float64(capacity)
			}

			err = srv.storage.Insert([]any{userVolume})
			if err != nil {
				log.Printf("failed to insert user volume: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert uv"})
				return
			}
			c.JSON(http.StatusCreated, gin.H{"status": "inserted user volume"})
			return
		}
		// binded user
		err = srv.storage.Insert([]any{userVolumes})
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
			userVolumes []ut.UserVolume
			data        interface{}
			err         error
		)

		if uids == "" && vids == "" {
			// return all
			data, err = srv.storage.Select("", "userVolume", "", "", 0)
			if err != nil {
				log.Printf("failed to retrieve user volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user volumes"})
				return
			}
			userVolumes = data.([]ut.UserVolume)
		} else if uids == "" {
			// return by vids
			data, err = srv.storage.Select("", "userVolume", "vid IN", vids, 0)
			if err != nil {
				log.Printf("failed to retrieve user volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user volumes"})
				return
			}
			userVolumes = data.([]ut.UserVolume)
		} else if vids == "" {
			// return by uids
			data, err = srv.storage.Select("", "userVolume", "uid IN", uids, 0)
			if err != nil {
				log.Printf("failed to retrieve user volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user volumes"})
				return
			}
			userVolumes = data.([]ut.UserVolume)
		}
		// else {
		// 	// return by both
		// 	data, err = srv.storage.Select(strings.Split(strings.TrimSpace(uids), ","), strings.Split(strings.TrimSpace(vids), ","))
		// 	if err != nil {
		// 		log.Printf("failed to retrieve user volumes: %v", err)
		// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user volumes"})
		// 		return
		// 	}
		// 	userVolumes = data.([]ut.UserVolume)
		// }
		c.JSON(http.StatusOK, gin.H{"content": userVolumes})

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

func (srv *UService) HandleGroupVolumes(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodPost:
		var (
			groupVolumes []ut.GroupVolume
			groupVolume  ut.GroupVolume
		)

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("failed to read request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}

		err = json.Unmarshal(body, &groupVolumes)
		if err != nil {
			log.Printf("didn't bind groupVolumes, lets try a groupVolume..")
			err = json.Unmarshal(body, &groupVolume)
			// single userVolume
			if err != nil {
				log.Printf("fail to bind body: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad request, failed to bind"})
				return
			}

			if capacity, _ := strconv.ParseFloat(srv.config.V_DEFAULT_CAPACITY, 64); int(groupVolume.Quota) == 0 || groupVolume.Quota > float64(capacity) {
				log.Printf("inserted")
				groupVolume.Quota = float64(capacity)
			}

			err = srv.storage.Insert([]any{groupVolume})
			if err != nil {
				log.Printf("failed to insert group volume: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert gv"})
				return
			}
			c.JSON(http.StatusCreated, gin.H{"status": "inserted group volume"})
			return
		}

		// binded user
		err = srv.storage.Insert([]any{groupVolumes})
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
			groupVolumes []ut.GroupVolume
			data         interface{}
			err          error
		)

		if gids == "" && vids == "" {
			// return all
			data, err = srv.storage.Select("", "groupVolume", "", "", 0)
			if err != nil {
				log.Printf("failed to retrieve group volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve group volumes"})
				return
			}
			groupVolumes = data.([]ut.GroupVolume)
		} else if gids == "" {
			// return by vids
			data, err = srv.storage.Select("", "groupVolume", "vid", vids, 0)
			if err != nil {
				log.Printf("failed to retrieve group volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve group volumes"})
				return
			}
			groupVolumes = data.([]ut.GroupVolume)
		} else if vids == "" {
			// return by gids
			data, err = srv.storage.Select("", "groupVolume", "gid", gids, 0)
			if err != nil {
				log.Printf("failed to retrieve group volumes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve group volumes"})
				return
			}
			groupVolumes = data.([]ut.GroupVolume)
		}
		// else {
		// 	// return by both
		// 	data, err = srv.storage.GetGroupVolumesByVidsAndGids(strings.Split(strings.TrimSpace(vids), ","), strings.Split(strings.TrimSpace(gids), ","))
		// 	if err != nil {
		// 		log.Printf("failed to retrieve group volumes: %v", err)
		// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retireve group volumes"})
		// 		return
		// 	}
		// 	groupVolumes = data.([]ut.GroupVolume)
		// }
		c.JSON(http.StatusOK, gin.H{"content": groupVolumes})

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

// these two funcs seem to be irrelevant here.. should belong to fslite
