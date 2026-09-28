package uspace

/*
	http api handlers for the uspace service
	"volume" related endpoints
*/

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

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
	ctx := c.Request.Context()
	switch c.Request.Method {
	case http.MethodGet:
		// fslite's records: they carry the ids, capacity and usage (the
		// object store's bucket list carried none of it)
		volumes, err := srv.fsl.SelectVolumes(ctx, map[string]any{"vid": c.Query("vid")})
		if err != nil {
			log.Printf("[USPACE_API] failed to list volumes: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list volumes"})

			return
		}
		list, ok := volumes.([]ut.Volume)
		if v, single := volumes.(ut.Volume); single {
			list, ok = []ut.Volume{v}, true
		}
		if !ok {
			c.JSON(http.StatusOK, gin.H{"content": volumes})

			return
		}
		// mark group volumes
		if gvs, err := srv.fsl.GroupVolumes(ctx, nil); err == nil {
			byVID := map[int64]int64{}
			for _, gv := range gvs {
				byVID[gv.VID] = gv.GID
			}
			for i := range list {
				if gid, shared := byVID[list[i].VID]; shared {
					list[i].Shared, list[i].GID = true, gid
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"content": list})

	case http.MethodPost:
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read req body"})

			return
		}
		var volumes []ut.Volume
		if json.Unmarshal(body, &volumes) != nil {
			var one ut.Volume
			if err := json.Unmarshal(body, &one); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})

				return
			}
			volumes = []ut.Volume{one}
		}
		for _, v := range volumes {
			if err := v.Validate(srv.config.LocalVolumesDefaultCapacity, srv.config.LocalVolumesDefaultCapacity, "-."); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

				return
			}
			if err := srv.createVolume(ctx, v); err != nil {
				respondErr(c, "create volume "+v.Name, err)

				return
			}
		}
		c.JSON(http.StatusCreated, gin.H{"message": "inserted volume(s) successfully"})

	case http.MethodDelete:
		name := c.Query("volume")
		switch name {
		case "":
			c.JSON(http.StatusBadRequest, gin.H{"error": "must provide a volume name"})

			return
		case srv.config.MinioDefaultBucket:
			c.JSON(http.StatusForbidden, gin.H{"error": "the default volume can't be deleted"})

			return
		}
		if err := srv.storage.RemoveVolume(ctx, name); err != nil {
			respondErr(c, "delete the volume", err) // 409 while it holds files

			return
		}
		if err := srv.fsl.RemoveVolume(ctx, ut.Volume{Name: name}); err != nil {
			log.Printf("[USPACE_API] bucket %s deleted but its record stays: %v", name, err)
		}
		c.JSON(http.StatusAccepted, gin.H{"status": "successfully deleted volume"})

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "not allowed."})
	}
}

// createVolume records a volume in fslite and creates it in the object
// store. It used to create only the bucket, so the volume had no record
// and every upload to it failed the resources' foreign key. A bucket that
// already exists is adopted (volumes made the old way can be re-created).
func (srv *UService) createVolume(ctx context.Context, v ut.Volume) error {
	v.CreatedAt = ut.CurrentTime()
	if err := srv.fsl.CreateVolume(ctx, v); err != nil {
		return err
	}
	if err := srv.storage.CreateVolume(ctx, v); err != nil && !errors.Is(err, ut.ErrExists) {
		if rbErr := srv.fsl.RemoveVolume(context.WithoutCancel(ctx), v); rbErr != nil {
			log.Printf("[USPACE_API] failed to roll back volume record %s: %v", v.Name, rbErr)
		}

		return err
	}

	return nil
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
// @Router /admin/user/volume [get]
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

			// callers don't know volume ids (frontapp sends 1): fall back to
			// the default volume when the given one doesn't exist
			if v, err := srv.fsl.SelectVolumes(c.Request.Context(), map[string]any{"vid": strconv.FormatInt(userVolume.VID, 10)}); err != nil || v == nil {
				userVolume.VID = srv.volumeID(c.Request.Context(), srv.config.MinioDefaultBucket)
			}
			if capacity := srv.config.LocalVolumesDefaultCapacity; int(userVolume.Quota) == 0 || userVolume.Quota > capacity {
				userVolume.Quota = capacity
			}

			// fslite's Insert switches on bare ut.UserVolume/[]ut.UserVolume
			// (not []any) and returns a nil cancel func for them.
			err := srv.fsl.Insert(c.Request.Context(), userVolume)
			if err != nil {
				log.Printf("[USPACE_API] failed to insert user volume: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert uv"})

				return
			}
			c.JSON(http.StatusCreated, gin.H{"status": "inserted user volume"})

			return
		}
		// binded user
		err = srv.fsl.Insert(c.Request.Context(), userVolumes)
		if err != nil {
			log.Printf("[USPACE_API] failed to insert user volumes: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert uv"})

			return
		}
		c.JSON(http.StatusCreated, gin.H{"status": "inserted user volumes"})
	case http.MethodGet:
		// ?uids=1,2 &volume=name: claims with their computed usage
		var uids []int64
		if raw := c.Query("uids"); raw != "" {
			uids = callerGroups(ut.AccessClaim{Gids: raw}) // same "1,2,3" parsing
		}
		uvs, err := srv.fsl.UserVolumes(c.Request.Context(), uids, c.Query("volume"))
		if err != nil {
			respondErr(c, "list user quotas", err)

			return
		}
		c.JSON(http.StatusOK, uvs)
	case http.MethodPatch:
		var req struct {
			Vname string  `json:"vname" binding:"required"`
			UID   int64   `json:"uid"   binding:"required"`
			Quota float64 `json:"quota"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "need vname, uid and quota: " + err.Error()})

			return
		}
		uv, err := srv.fsl.SetUserQuota(c.Request.Context(), req.Vname, req.UID, req.Quota)
		if err != nil {
			respondErr(c, "set the quota", err)

			return
		}
		c.JSON(http.StatusOK, uv)
	case http.MethodDelete:
		uid, err := strconv.ParseInt(c.Query("uid"), 10, 64)
		if err != nil || c.Query("volume") == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "need ?volume= and ?uid="})

			return
		}
		if err := srv.fsl.ResetUserQuota(c.Request.Context(), c.Query("volume"), uid); err != nil {
			respondErr(c, "reset the quota", err)

			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "quota reset to the default"})

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

// these two funcs seem to be irrelevant here.. should belong to fslite
