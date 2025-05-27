package uspace

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

// handleApps handles application registration and retrieval for regular users.
//
// @Summary Application endpoint (user)
// @Description Retrieves or registers applications. Accepts queries or full application POST payloads.
// @Tags apps
//
// @Accept json
// @Produce json
//
// @Param limit query string false "Pagination limit for listing applications"
// @Param offset query string false "Pagination offset for listing applications"
// @Param ids query string false "Comma-separated application IDs to filter"
// @Param name query string false "Application name to fetch specific app"
// @Param version query string false "Application version to fetch specific app"
// @Param app body ut.Application true "Single application object (for POST)"
// @Param apps body []ut.Application true "Multiple applications (for POST)"
//
// @Success 200 {object} map[string]interface{} "Success with application(s) content or status"
// @Failure 400 {object} map[string]string "Bad request (e.g., malformed input)"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 405 {object} map[string]string "Method not allowed"
//
// @Router /app [get]
// @Router /app [post]
func (srv *UService) handleApps(c *gin.Context) {
	var (
		app  ut.Application
		apps []ut.Application
	)
	switch c.Request.Method {
	// "getting" jobs should be treated as "subscribing"
	case http.MethodGet:
		limit, _ := c.GetQuery("limit")
		offset, _ := c.GetQuery("offset")
		ids, _ := c.GetQuery("ids")
		if ids != "" {
			// return all jobs from database by uids
			ids_int, err := ut.SplitToInt(ids, ",")
			if err != nil {
				log.Printf("failed to atoi ids: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi ids"})
				return
			}
			apps, err := srv.getAppsByIds(ids_int)
			if err != nil {
				log.Printf("failed to retrieve apps by id: %v, %v", ids_int, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve apps by id"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"content": apps})
			return
		}

		name, _ := c.GetQuery("name")
		version, _ := c.GetQuery("version")
		if name == "" || version == "" {
			// return all jobs from database
			apps, err := srv.getAllApps(limit, offset)
			if err != nil {
				log.Printf("failed to retrieve the apps: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the apps"})
				return
			}
			// log.Printf("jobs retrieved from db: %+v", jobs)
			c.JSON(http.StatusOK, gin.H{"content": apps})
			return
		}

		app, err := srv.getAppByNameAndVersion(name, version)
		if err != nil {
			log.Printf("failed to retrieve the app: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the app"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": app})

	case http.MethodPost:
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("failed to read request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}

		if err = json.Unmarshal(body, &app); err != nil {
			if err = json.Unmarshal(body, &apps); err != nil {
				log.Printf("failed to bind app(s): %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind app(s)"})
				return
			}
			err := srv.insertApps(apps)
			if err != nil {
				log.Printf("failed to save apps in the db: %+v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert into db"})
				return
			}

			// respond with status
			c.JSON(http.StatusOK, gin.H{
				"status": "app(s) published",
			})
			return
		}
		id, err := srv.insertApp(app)
		if err != nil {
			log.Printf("failed to insert the app in the db: %+v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert into db"})
			return
		}
		app.Id = id
		log.Printf("[Database] app id acquired: %d", id)

		// respond with status
		c.JSON(http.StatusOK, gin.H{
			"status": "app inserted",
			"id":     id,
		})

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "method not allowed",
		})
	}
}

// handleAppsAdmin provides admin-level control over application records.
//
// @Summary Application admin endpoint
// @Description Admin operations for applications: retrieve, insert, update, delete.
// @Tags admin, apps
//
// @Accept json
// @Produce json
//
// @Param limit query string false "Pagination limit for listing applications"
// @Param offset query string false "Pagination offset for listing applications"
// @Param ids query string false "Comma-separated application IDs to fetch or delete"
// @Param name query string false "Application name (used with version)"
// @Param version query string false "Application version (used with name)"
// @Param id query string false "Single application ID to delete"
// @Param app body ut.Application true "Single application object (POST/PUT)"
// @Param apps body []ut.Application true "Multiple application objects (POST)"
//
// @Success 200 {object} map[string]interface{} "Success with application(s) content or status"
// @Failure 400 {object} map[string]string "Bad request (e.g., malformed input)"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 405 {object} map[string]string "Method not allowed"
//
// @Router /admin/app [get]
// @Router /admin/app [post]
// @Router /admin/app [put]
// @Router /admin/app [delete]
func (srv *UService) handleAppsAdmin(c *gin.Context) {
	var (
		app  ut.Application
		apps []ut.Application
	)
	switch c.Request.Method {
	case http.MethodGet:
		limit, _ := c.GetQuery("limit")
		offset, _ := c.GetQuery("offset")
		ids, _ := c.GetQuery("ids")
		if ids != "" {
			// return all jobs from database by uids
			ids_int, err := ut.SplitToInt(ids, ",")
			if err != nil {
				log.Printf("failed to atoi ids: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to atoi ids"})
				return
			}
			apps, err := srv.getAppsByIds(ids_int)
			if err != nil {
				log.Printf("failed to retrieve apps by id: %v, %v", ids_int, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve apps by id"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"content": apps})
			return
		}

		name, _ := c.GetQuery("name")
		version, _ := c.GetQuery("version")
		if name == "" || version == "" {
			// return all jobs from database
			apps, err := srv.getAllApps(limit, offset)
			if err != nil {
				log.Printf("failed to retrieve the apps: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the apps"})
				return
			}
			// log.Printf("jobs retrieved from db: %+v", jobs)
			c.JSON(http.StatusOK, gin.H{"content": apps})
			return
		}

		app, err := srv.getAppByNameAndVersion(name, version)
		if err != nil {
			log.Printf("failed to retrieve the app: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve the app"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": app})
	case http.MethodPost:
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("failed to read request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}

		if err = json.Unmarshal(body, &app); err != nil {
			if err = json.Unmarshal(body, &apps); err != nil {
				log.Printf("failed to bind app(s): %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind app(s)"})
				return
			}

			err := srv.insertApps(apps)
			if err != nil {
				log.Printf("failed to save apps in the db: %+v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert into db"})
				return
			}

			// respond with status
			c.JSON(http.StatusOK, gin.H{
				"status": "app(s) published",
			})
			return
		}
		// save job (insert in DB)
		id, err := srv.insertApp(app)
		if err != nil {
			log.Printf("failed to insert the app in the db: %+v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert into db"})
			return
		}
		app.Id = id
		log.Printf("[Database] app id acquired: %d", id)

		// respond with status
		c.JSON(http.StatusOK, gin.H{
			"status": "app inserted",
			"id":     id,
		})

	case http.MethodPut:
		err := c.BindJSON(&app)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind data"})
			return
		}
		if app.Id == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "must specify an id value"})
			return
		}

		err = srv.updateApp(app)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update app"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "update success"})

	case http.MethodDelete:
		id := c.Query("id")
		if id != "" {
			id_int, err := strconv.Atoi(id)
			if err != nil {
				log.Printf("failed to atoi id")
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad id format"})
				return
			}
			err = srv.removeApp(id_int)
			if err != nil {
				log.Printf("failed to remove app")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove app"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "sucess"})
			return
		}
		ids := c.Query("ids")
		if ids != "" {
			ids_int, err := ut.SplitToInt(strings.TrimSpace(ids), ",")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad ids format"})
				return
			}
			err = srv.removeApps(ids_int)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove apps"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "success"})
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "must specify an argument"})

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "method not allowed",
		})
	}
}
