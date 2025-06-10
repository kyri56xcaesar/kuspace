// Package fslite provides a lightweight file system abstraction layer with support for
// volume management, resource (file/object) management, and user-volume quota tracking.
// It is designed to work with both local file storage and database-backed metadata,
// supporting operations such as creating, removing, copying, and querying volumes and resources.
//
// The FsLite struct is the main entry point, encapsulating configuration, database handler,
// and optionally a Gin engine for API exposure. The package supports initialization with
// default volumes, admin user setup, and flexible storage locality.
//
// Key Features:
//   - Volume management: create, remove, query, and manage storage volumes with capacity limits.
//   - Resource management: insert, remove, copy, and query files/objects, with optional local storage.
//   - User-volume quota: track and enforce per-user storage usage and quotas.
//   - Database-backed metadata: all operations are tracked in a relational database.
//   - Locality support: optionally store files on the local filesystem, or operate in metadata-only mode.
//   - Extensible API: designed for integration with web APIs via Gin.
//
// Main Types and Functions:
//
//   - FsLite: Core struct managing configuration, database, and API engine.
//   - NewFsLite: Initializes a new FsLite instance, sets up database schema, admin user, and default volume.
//   - CreateVolume, RemoveVolume, SelectVolumes: Manage storage volumes.
//   - Insert, Remove, SelectObjects: Manage resources (files/objects).
//   - claimVolumeSpace, releaseVolumeSpace: Track and update storage usage and quotas.
//   - Download, Copy: Support for file download and duplication.
//   - selectUserVolumes: Query user-volume usage and quota information.
//   - determinePhysicalStorage: Ensures physical storage paths exist and have sufficient space.
//
// Usage:
//
//   - Instantiate FsLite with configuration using NewFsLite.
//   - Use provided methods to manage volumes, resources, and user quotas.
//   - Integrate with Gin for API exposure if required.
//
// Note: This package depends on internal utility types and functions (ut package) for
// configuration, database handling, and common operations. Ensure these dependencies are available.
//
// Example:
//
//	cfg := ut.EnvConfig{...}
//	fsl := fslite.NewFsLite(cfg)
//	err := fsl.CreateVolume(ut.Volume{Name: "myvol", Capacity: 10})
//	...
package fslite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

var (
	fsliteDataPath            = "data/volumes/fslite"
	defaultVolumeName         = "default_ku_space_volume"
	verbose                   = false
	unlocked                  = false
	defaultVolumeCap  float64 = 20
	maxVolumeCap      float64 = 100
)

const (
	initSQL = `
		CREATE TABLE IF NOT EXISTS user_admin (
			uuid TEXT PRIMARY KEY,
			username TEXT,
			hashpass TEXT
		);
    	CREATE TABLE IF NOT EXISTS resources (
    	  rid INTEGER PRIMARY KEY,
    	  uid INTEGER,
    	  gid INTEGER,
    	  vid INTEGER,
		  vname TEXT,
    	  size BIGINT,
    	  links INTEGER,
    	  perms TEXT,
    	  name TEXT,
		  path TEXT,
    	  type TEXT,
    	  createdAt DATETIME,
    	  updatedAt DATETIME,
    	  accessedAt DATETIME
    	);
    	CREATE TABLE IF NOT EXISTS volumes (
    	  vid INTEGER PRIMARY KEY,
    	  name TEXT,
    	  path TEXT,
		  dynamic BOOLEAN,
    	  capacity FLOAT,
    	  usage FLOAT,
		  createdAt DATETIME
    	);
		CREATE TABLE IF NOT EXISTS userVolume(
			vid INTEGER,
			uid INTEGER,
			usage FLOAT,
			quota FLOAT,
			updatedAt DATETIME
		);
    	CREATE SEQUENCE IF NOT EXISTS seqResourceId START 1;
    	CREATE SEQUENCE IF NOT EXISTS seqVolumeId START 1; 
    `
)

// FsLite central object
// a database handler
// a server engine (gin)
// a configuration setup
type FsLite struct {
	config ut.EnvConfig
	dbh    ut.DBHandler
	// Engine exported for testing
	Engine *gin.Engine
}

// NewFsLite initializes a new FsLite instance with the provided configuration.
// It sets up the database schema, creates the admin user, and ensures the default volume exists.
// If FslLocality is enabled, it also prepares the local storage directory.
// Returns the initialized FsLite struct.
func NewFsLite(cfg ut.EnvConfig) FsLite {
	setGinMode(cfg.APIGinMode)
	var ginEngine *gin.Engine
	if cfg.FslServer {
		ginEngine = gin.Default()
	}
	fsl := FsLite{
		config: cfg,
		dbh:    ut.NewDBHandler(cfg.FslDB, cfg.FslDBPath, cfg.FslDBDriver),
		Engine: ginEngine,
	}
	fsl.dbh.Init(initSQL, cfg.FslDBMaxOpenConns, cfg.FslDBMaxIdleConns, cfg.FslDBMaxLifetime)
	if _, err := fsl.insertAdmin(cfg.FslAccessKey, cfg.FslSecretKey); err != nil {
		log.Fatalf("[FSL_init] error inserting main user, fatal...: %v", err)
	}
	if fsl.config.FslLocality {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("[FSL_init] failed to get working directory: %v", err)
		}
		fsliteDataPath = wd + "/" + cfg.LocalVolumesDefaultPath
		if err := os.MkdirAll(fsliteDataPath, 0o644); err != nil {
			log.Fatalf("[FSL_init] failed to create main volume storage path: %v", err)
		}
	}
	defaultVolumeCap = cfg.LocalVolumesDefaultCapacity
	defaultVolumeCap = min(defaultVolumeCap, maxVolumeCap)
	err := fsl.CreateVolume(ut.Volume{Name: defaultVolumeName, Path: fsliteDataPath + "/" + defaultVolumeName, Capacity: defaultVolumeCap})
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			log.Fatalf("[FSL_init] failed to create default volume: %v", err)
		}
		log.Print(err)
	}
	JwtValidityHours = cfg.JwtValidityHours
	verbose = cfg.Verbose

	return fsl
}

// Close closes the underlying database handler and releases any resources held by FsLite.
func (fsl *FsLite) Close() {
	fsl.dbh.Close()
}

// CreateVolume creates a new storage volume based on the provided ut.Volume struct.
// It validates the volume, ensures uniqueness, and creates the physical directory if locality is enabled.
func (fsl *FsLite) CreateVolume(v any) error {
	volume, ok1 := v.(ut.Volume)
	if !ok1 {
		return errors.New("failed to cast to volume")
	}
	if err := volume.Validate(maxVolumeCap, defaultVolumeCap, "-._"); err != nil {
		return err
	}

	db, err := fsl.dbh.GetConn()
	if err != nil {
		return errors.New("failed to get the database connection")
	}

	// should check if name exists.
	if _, err = getVolumeByName(db, volume.Name); err == nil { // if err is nil, it exists
		return ut.NewInfo("%s volume already exists", volume.Name)
	} else if err.Error() != "empty" {
		return err
	}

	if fsl.config.FslLocality {
		err = os.MkdirAll(fsliteDataPath+"/"+volume.Name, 0o644)
		if err != nil {
			return err
		}
	}

	volume.CreatedAt = ut.CurrentTime()
	err = insertVolume(db, volume)
	if err != nil {
		err1 := os.RemoveAll(fsliteDataPath + "/" + volume.Name)
		if err1 != nil {
			log.Printf("failed to remove path: %v", err)
		}

		return err
	}

	return err
}

// DefaultVolume returns the name of the default volume.
// The 'local' parameter is currently unused.
func (fsl *FsLite) DefaultVolume(_ bool) string {
	return defaultVolumeName
}

// RemoveVolume removes a storage volume specified by the ut.Volume struct.
// It deletes the volume from the database and removes the physical directory if locality is enabled.
func (fsl *FsLite) RemoveVolume(t any) error {
	volume, ok := t.(ut.Volume)
	if !ok {
		return errors.New("failed to cast to volume")
	}

	if err := volume.Validate(maxVolumeCap, defaultVolumeCap, "-._"); err != nil {
		return err
	}

	db, err := fsl.dbh.GetConn()
	if err != nil {
		return err
	}
	if volume.Name != "" {
		err = deleteVolumeByName(db, volume.Name)
	} else {
		err = deleteVolume(db, volume.VID)
	}

	if err == nil && fsl.config.FslLocality {
		err = os.RemoveAll(fsliteDataPath + "/" + volume.Name)
	}

	return err
}

// SelectVolumes queries and returns volumes based on the provided filter map.
// Supports filtering by name or vid (volume ID). Returns all volumes if no filter is provided.
func (fsl *FsLite) SelectVolumes(how map[string]any) (any, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("[FSL_select_volume(s)] failed to get the db conn: %v", err)

		return nil, err
	}
	// limit := how["limit"]

	vname, ok := how["name"]
	if ok && vname != "" {
		name, ok := vname.(string)
		if ok {
			return getVolumeByName(db, name)
		}
	}

	vid, ok := how["vid"]
	if ok && vid != "" {
		vid, err := strconv.Atoi(vid.(string))
		if err == nil {
			return getVolumeByVid(db, vid)
		}

		return nil, err
	}

	return getAllVolumes(db)
}

// Insert inserts resources (files/objects), user-volume records, or batches thereof into the database.
// If inserting a resource and locality is enabled, it also writes the file to disk.
func (fsl *FsLite) Insert(t any) (context.CancelFunc, error) {
	resource, ok := t.(ut.Resource)
	if ok {
		if resource.Vname == "" {
			resource.Vname = defaultVolumeName
		}

		db, err := fsl.dbh.GetConn()
		if err != nil {
			log.Printf("[FSL_insert] failed to get the db conn: %v", err)

			return nil, ut.NewError("failed to get the db conn: %v", err)
		}

		exists, err := exists(db, resource.Name, resource.Vname)
		if err != nil { // if err is nil, it exists
			log.Printf("[FSL_insert] failed to check if object exists")

			return nil, ut.NewError("failed to check if obj exists: %v", err)
		}
		if exists {
			return nil, ut.NewInfo("%s object already exists", resource.Name)
		}

		if fsl.config.FslLocality {
			outFile, err := os.Create(fsliteDataPath + "/" + resource.Vname + "/" + resource.Name)
			if err != nil {
				log.Printf("[FSL_insert] failed to create a new output file (to save)")

				return nil, err
			}
			defer func() {
				err := outFile.Close()
				if err != nil {
					log.Printf("failed to close the file: %v", err)
				}
			}()

			// Copy from the reader to the file
			_, err = io.Copy(outFile, resource.Reader)
			if err != nil {
				log.Printf("[FSL_insert] failed to copy to output file")

				return nil, err
			}
		}
		// db
		err = insertResource(db, resource)
		if err != nil {
			log.Printf("[FSL_insert] failed to insert resources in the db: %v", err)

			return nil, err
		}

		return nil, err
	}
	resources, ok := t.([]ut.Resource)
	if ok {
		db, err := fsl.dbh.GetConn()
		if err != nil {
			log.Printf("[FSL_insert] failed to get the db conn: %v", err)

			return nil, err
		}
		err = insertResources(db, resources)
		if err != nil {
			log.Printf("[FSL_insert] failed to insert resources in the db: %v", err)

			return nil, err
		}

		return nil, err
	}

	uv, ok := t.(ut.UserVolume)
	if ok {
		db, err := fsl.dbh.GetConn()
		if err != nil {
			log.Printf("[FSL_insert] failed to get the db conn: %v", err)

			return nil, err
		}
		err = insertUserVolume(db, uv)

		return nil, err
	}
	uvs, ok := t.([]ut.UserVolume)
	if ok {
		db, err := fsl.dbh.GetConn()
		if err != nil {
			log.Printf("[FSL_insert] failed to get the db conn: %v", err)

			return nil, err
		}
		err = insertUserVolumes(db, uvs)

		return nil, err
	}

	return nil, errors.New("failed to cast all types")
}

// SelectObjects queries and returns resources (files/objects) based on the provided filter map.
// Supports filtering by prefix or resource IDs. Returns all resources if no filter is provided.
func (fsl *FsLite) SelectObjects(how map[string]any) (any, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("[FSL_select_objects] failed to get the db conn: %v", err)

		return nil, err
	}
	// limit := how["limit"]
	name, ok := how["prefix"]
	if ok && name != "" {
		name, ok := name.(string)
		if ok {
			return getResourcesByNameLike(db, name)
		}
	}
	rids, ok := how["rids"]
	if ok && rids != "" {
		rids, err := ut.SplitToInt(rids.(string), ",")
		if err == nil {
			return getResourcesByIDs(db, rids)
		}

		return nil, err
	}

	name, ok = how["name"]
	volume, ok2 := how["volume"]
	log.Printf("name: %v. volume: %v, ok: %v, ok2: %v", name, volume, ok, ok2)
	if ok && ok2 {
		return getResourceByNameAndVolume(db, name.(string), volume.(string))
	}

	return getAllResources(db)
}

// Stat returns file information for a resource if locality is enabled.
// Returns an error if locality is disabled or if the resource cannot be found.
func (fsl *FsLite) Stat(t any) (any, error) {
	if fsl.config.FslLocality {
		return nil, errors.New("cannot use stat if locality is turned off")
	}
	resource, ok := t.(ut.Resource)
	if !ok {
		log.Printf("[FSL_stat] failed to cast to designated struct")

		return nil, errors.New("failed to cast to designated struct")
	}

	return os.Stat(fsliteDataPath + "/" + resource.Vname + "/" + resource.Name)
}

// Remove deletes a resource (file/object) from the database and, if locality is enabled, from disk.
func (fsl *FsLite) Remove(t any) error {
	resource, ok := t.(ut.Resource)
	if !ok {
		log.Printf("[FSL_remove] failed to cast to designated struct")

		return errors.New("failed to cast to designated struct")
	}
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("[FSL_remove] failed to retrieve database connection: %v", err)

		return fmt.Errorf("failed to retrieve the database conn: %w", err)
	}

	err = deleteResourceByNameAndVolume(db, resource.Name, resource.Vname)
	if err != nil {
		log.Printf("[FSL_remove] failed to remove the resource from the database")

		return fmt.Errorf("failed to remove the resource properly: %w", err)
	}

	if fsl.config.FslLocality {
		err = os.Remove(fsliteDataPath + "/" + resource.Vname + "/" + resource.Name)
		if err != nil {
			log.Printf("[FSL_remove] failed to remove file from local fs")

			return fmt.Errorf("failed to remove the file locally: %w", err)
		}
	}

	return nil
}

// Update updates resource metadata such as permissions, owner, group, or renames a resource.
// The update is based on the provided map of string keys and values.
func (fsl *FsLite) Update(t map[string]string) error {
	if t == nil {
		return errors.New("empty argument")
	}
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("[FSL_update] failed to retrieve database connection: %v", err)

		return err
	}
	rid, exists := t["rid"]
	if exists {
		// either perms/owner/group
		perms, ok := t["perms"]
		if ok {
			return updateResourcePermsByID(db, rid, perms)
		}

		owner, ok := t["owner"]
		if ok {
			// atoi
			ridInt, err := strconv.Atoi(rid)
			if err != nil {
				return errors.New("failed to atoi rid")
			}
			ownerInt, err := strconv.Atoi(owner)
			if err != nil {
				return errors.New("failed to atoi uid")
			}

			return updateResourceOwnerByID(db, ridInt, ownerInt)
		}

		group, ok := t["group"]
		if ok {
			// atoi
			ridInt, err := strconv.Atoi(rid)
			if err != nil {
				return errors.New("failed to atoi rid")
			}
			groupInt, err := strconv.Atoi(group)
			if err != nil {
				return errors.New("failed to atoi gid")
			}

			return updateResourceGroupByID(db, ridInt, groupInt)
		}
	}

	name, ok1 := t["name"]
	newname, ok2 := t["newname"]
	volume, ok3 := t["volume"]
	if ok1 && ok2 && ok3 {
		return updateResourceNameAndVolByName(db, name, newname, volume)
	}

	return errors.New("must specify what to update")
}

// Download prepares a resource for download by opening the file and attaching a reader to the resource struct.
// Returns an error if locality is disabled or if the file cannot be found.
func (fsl *FsLite) Download(t *any) (context.CancelFunc, error) {
	if fsl.config.FslLocality {
		return nil, errors.New("cannot download if locality is off")
	}
	v := *t
	resource, ok := v.(ut.Resource)
	if !ok {
		log.Printf("[FSL_download] failed to cast to ut.Resource")

		return nil, errors.New("failed to cast to ut.Resource")
	}
	resourcePtr := &resource
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("[FSL_download] failed to retrieve database connection: %v", err)

		return nil, err
	}

	_, err = getResourceByNameAndVolume(db, resourcePtr.Name, resourcePtr.Vname)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fsliteDataPath + "/" + resourcePtr.Vname + "/" + resourcePtr.Name)
	if err != nil {
		return nil, err
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stats: %w", err)
	}
	resourcePtr.Size = stat.Size()
	resourcePtr.Reader = file

	*t = *resourcePtr

	return nil, nil
}

// Copy duplicates a resource (file/object) from a source to a destination, both in the database and on disk if locality is enabled.
func (fsl *FsLite) Copy(s, d any) error {
	src, ok := s.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}
	dst, ok := d.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("[FSL_copy] failed to retrieve database connection: %v", err)

		return err
	}

	if fsl.config.FslLocality {
		sr, err := os.Open(fsliteDataPath + "/" + src.Vname + "/" + src.Name)
		if err != nil {
			log.Printf("[FSL_copy] failed to read the src file")

			return err
		}
		defer func() {
			err := sr.Close()
			if err != nil {
				log.Printf("failed to close the file: %v", err)
			}
		}()
		sr1, err := io.ReadAll(sr)
		if err != nil {
			log.Printf("[FSL_copy] failed to read the src file to a buffer")

			return err
		}

		ds, err := os.OpenFile(fsliteDataPath+"/"+dst.Vname+"/"+dst.Name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			log.Printf("[FSL_copy] failed to open the dst file")

			return err
		}
		defer func() {
			err := ds.Close()
			if err != nil {
				log.Printf("failed to close the file: %v", err)
			}
		}()

		_, err = ds.Write(sr1)
		if err != nil {
			log.Printf("[FSL_copy] failed to write to output file")
		}
	}

	// update db
	err = insertResource(db, dst)
	if err != nil {
		log.Printf("[FSL_copy] failed to insert to the db0")
	}

	return err
}

// Share is a placeholder for sharing functionality. Currently unimplemented.
func (fsl *FsLite) Share(_ string, _ any) (any, error) {
	return nil, nil
}

func (fsl *FsLite) claimVolumeSpace(size int64, volumeName, uid string) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("[FSL_claim] failed to retrieve database connection: %v", err)

		return err
	}

	volume, err := getVolumeByName(db, volumeName)
	if err != nil {
		return err
	}
	sizeInGB := ut.SizeInGb(size)
	// check for current volume usage.
	newUsageInGB := volume.Usage + sizeInGB
	if newUsageInGB > volume.Capacity {
		log.Printf("[FSL_claim] volume is full.")

		return errors.New("claim exceeds capacity")
	}

	// if not dynamic, we should check for per user/group quota
	iuid, err := strconv.Atoi(uid)
	if err != nil {
		return err
	}
	// if it doesn't exist, create it
	uv, err := getUserVolumeByUID(db, iuid)
	if err != nil {
		err = insertUserVolume(db, ut.UserVolume{UpdatedAt: ut.CurrentTime(), VID: volume.VID, UID: iuid, Usage: sizeInGB})
		if err != nil {
			log.Printf("[FSL_claim] failed to insert uv ")

			return err
		}
	}

	// update all usages
	// volume
	// claims user/group
	uv.Usage += sizeInGB
	volume.Usage = newUsageInGB

	err = updateVolume(db, volume)
	if err != nil {
		log.Printf("[FSL_claim] failed to update volume usages: %v", err)

		return err
	}
	err = updateUserVolume(db, uv)
	if err != nil {
		log.Printf("[FSL_claim] failed to update user volume usages: %v", err)

		return err
	}

	return nil
}

func (fsl *FsLite) releaseVolumeSpace(size int64, volumeName, uid string) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("[FSL_release] failed to retrieve database connection: %v", err)

		return err
	}

	volume, err := getVolumeByName(db, volumeName)
	if err != nil {
		log.Printf("[FSL_release] could not retrieve volume: %v", err)

		return fmt.Errorf("could not retrieve volume: %w", err)
	}

	sizeInGB := ut.SizeInGb(size)
	newUsageInGB := max(volume.Usage-sizeInGB, 0)

	iuid, err := strconv.Atoi(uid)
	if err != nil {
		return err
	}
	uv, err := getUserVolumeByUID(db, iuid)
	if err != nil {
		return err
	}

	// update all usages
	// volume
	// claims user/group
	uv.Usage = max(0, uv.Usage-sizeInGB)
	volume.Usage = newUsageInGB

	err = updateVolume(db, volume)
	if err != nil {
		log.Printf("[FSL_release] failed to update volume usages: %v", err)

		return err
	}
	err = updateUserVolume(db, uv)
	if err != nil {
		log.Printf("[FSL_release] failed to update user volume usages: %v", err)

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
		return "", fmt.Errorf("failed to get available space: %w", err)
	}

	if fileSize < 0 || availableSpace < uint64(fileSize) {
		return "", errors.New("insufficient space")
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

func (fsl *FsLite) selectUserVolumes(how map[string]any) (any, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to get the db conn: %v", err)

		return nil, err
	}
	// limit := how["limit"]

	uids, ok1 := how["uids"].(string)
	vids, ok2 := how["vids"].(string)
	if ok1 && vids != "" && ok2 && uids != "" {
		// log.Printf("selecting all uvs by uids and vids")

		return getUserVolumesByUidsAndVids(db, strings.Split(uids, ","), strings.Split(vids, ","))
	} else if ok1 && vids != "" {
		// log.Printf("selecting uvs by vids: %v", vids)

		return getUserVolumesByVolumeIDs(db, strings.Split(vids, ","))
	} else if ok2 && uids != "" {
		// log.Printf("selecting uvs by uids")

		return getUserVolumesByUserIDs(db, strings.Split(uids, ","))
	}
	// log.Printf("selecting all uvs")

	return getAllUserVolumes(db)
}
