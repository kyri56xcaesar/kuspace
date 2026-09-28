package uspace

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"
	"kyri56xcaesar/kuspace/pkg/fslite"

	"github.com/gin-gonic/gin"
)

/*
	Group volumes (see pkg/fslite/group_volume.go): a volume that belongs to
	one group. Only its members (and root) write there; their files get the
	group as their group, so the other members can read them; the space is
	charged to the group's quota on the volume, not to the members' own.
*/

var errNotGroupMember = errors.New("only members of the volume's group can write to it")

// isMember reports whether the verified caller is in group gid (root always is).
func isMember(ac ut.AccessClaim, gid int64) bool {
	if ac.UID == "0" {
		return true
	}
	for _, g := range strings.Split(ac.Gids, ",") {
		if n, err := strconv.ParseInt(strings.TrimSpace(g), 10, 64); err == nil && n == gid {
			return true
		}
	}

	return false
}

// callerGroups is the caller's gids as numbers.
func callerGroups(ac ut.AccessClaim) []int64 {
	var gids []int64
	for _, g := range strings.Split(ac.Gids, ",") {
		if n, err := strconv.ParseInt(strings.TrimSpace(g), 10, 64); err == nil {
			gids = append(gids, n)
		}
	}

	return gids
}

// writeGroup is the group a file written by ac (as uid) to volume gets: the
// volume's group on a group volume (members only, else errNotGroupMember),
// otherwise the caller's primary group.
func (srv *UService) writeGroup(ctx context.Context, ac ut.AccessClaim, volume string, uid int64) (int64, error) {
	gv, err := srv.fsl.GroupVolume(ctx, volume)
	switch {
	case errors.Is(err, fslite.ErrNotGroupVolume):
		return primaryGID(ac, uid), nil
	case err != nil:
		return 0, err
	case !isMember(ac, gv.GID):
		return 0, errNotGroupMember
	}

	return gv.GID, nil
}

// respondWriteGroupError answers a writeGroup failure (403 or 500).
func respondWriteGroupError(c *gin.Context, err error) {
	if errors.Is(err, errNotGroupMember) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})

		return
	}
	log.Printf("failed to resolve the volume's group: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check the volume's group"})
}

// handleSharedVolumes lists the group volumes the caller can use (all of
// them for root).
//
// @Summary     List my group volumes
// @Tags        volumes
// @Produce     json
// @Success     200 {array} ut.GroupVolume
// @Router      /volumes/shared [get]
func (srv *UService) handleSharedVolumes(c *gin.Context) {
	ac, err := BindAccessTarget(c.GetHeader("Access-Target"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing caller identity"})

		return
	}
	var gids []int64
	if ac.UID != "0" {
		if gids = callerGroups(ac); len(gids) == 0 {
			c.JSON(http.StatusOK, []ut.GroupVolume{})

			return
		}
	}
	gvs, err := srv.fsl.GroupVolumes(c.Request.Context(), gids)
	if err != nil {
		log.Printf("failed to list group volumes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list group volumes"})

		return
	}
	c.JSON(http.StatusOK, gvs)
}

// groupVolumeRequest assigns a volume to a group (quota in GB, 0 = only
// the volume's capacity limits it).
type groupVolumeRequest struct {
	Vname string  `json:"vname" binding:"required"`
	GID   int64   `json:"gid" binding:"required"`
	Quota float64 `json:"quota"`
}

// handleGroupVolumes is the admin API for group volumes.
//
//	GET    ?gids=1,2          list (all without gids)
//	POST   {vname,gid,quota}  give an existing volume to a group / change its quota
//	PATCH  the same as POST
//	DELETE ?volume=name       make an empty group volume an ordinary volume
//
// @Summary     Manage group volumes
// @Tags        volumes
// @Accept      json
// @Produce     json
// @Router      /admin/group/volume [get]
// @Router      /admin/group/volume [patch]
// @Router      /admin/group/volume [post]
// @Router      /admin/group/volume [delete]
func (srv *UService) handleGroupVolumes(c *gin.Context) {
	ctx := c.Request.Context()
	switch c.Request.Method {
	case http.MethodGet:
		var gids []int64
		if raw := c.Query("gids"); raw != "" {
			gids = callerGroups(ut.AccessClaim{Gids: raw})
			if len(gids) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "gids must be numbers"})

				return
			}
		}
		gvs, err := srv.fsl.GroupVolumes(ctx, gids)
		if err != nil {
			log.Printf("failed to list group volumes: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list group volumes"})

			return
		}
		c.JSON(http.StatusOK, gvs)

	case http.MethodPost, http.MethodPatch:
		var req groupVolumeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "need vname and gid: " + err.Error()})

			return
		}
		if req.Vname == srv.config.MinioDefaultBucket {
			respondErr(c, "assign the volume", fslite.ErrDefaultVolume)

			return
		}
		gv, err := srv.fsl.AssignGroupVolume(ctx, req.Vname, req.GID, req.Quota)
		if err != nil {
			respondErr(c, "assign the volume", err)

			return
		}
		c.JSON(http.StatusOK, gv)

	case http.MethodDelete:
		volume := c.Query("volume")
		if volume == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "need ?volume="})

			return
		}
		if err := srv.fsl.ReleaseGroupVolume(ctx, volume); err != nil {
			respondErr(c, "release the group volume", err)

			return
		}
		c.JSON(http.StatusOK, gin.H{"status": volume + " is no longer a group volume"})

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}
