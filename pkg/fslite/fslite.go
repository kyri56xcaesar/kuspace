package fslite

import (
	"context"
	"fmt"
	"io"
	ut "kyri56xcaesar/kuspace/internal/utils"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	fslite_data_path    string  = "data/volumes/fslite"
	default_volume_name string  = "default_ku_space_volume"
	default_volume_cap  float64 = 20
	max_volume_cap      float64 = 100
)

const (
	InitSql = `
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
    	  created_at DATETIME,
    	  updated_at DATETIME,
    	  accessed_at DATETIME
    	);
    	CREATE TABLE IF NOT EXISTS volumes (
    	  vid INTEGER PRIMARY KEY,
    	  name TEXT,
    	  path TEXT,
		  dynamic BOOLEAN,
    	  capacity FLOAT,
    	  usage FLOAT,
		  created_at DATETIME
    	);
		CREATE TABLE IF NOT EXISTS userVolume(
			vid INTEGER,
			uid INTEGER,
			usage FLOAT,
			quota FLOAT,
			updated_at DATETIME
		);
    	CREATE SEQUENCE IF NOT EXISTS seq_resourceid START 1;
    	CREATE SEQUENCE IF NOT EXISTS seq_volumeid START 1; 
    `
)

type FsLite struct {
	config ut.EnvConfig
	dbh    ut.DBHandler
	engine *gin.Engine
}

func NewFsLite(cfg ut.EnvConfig) FsLite {
	setGinMode(cfg.API_GIN_MODE)
	var gin_engine *gin.Engine
	if cfg.FSL_SERVER {
		gin_engine = gin.Default()
	}
	fsl := FsLite{
		config: cfg,
		dbh:    ut.NewDBHandler(cfg.DB_FSL, cfg.DB_FSL_PATH, cfg.DB_FSL_DRIVER),
		engine: gin_engine,
	}
	fsl.dbh.Init(InitSql, cfg.DB_FSL_MAX_OPEN_CONNS, cfg.DB_FSL_MAX_IDLE_CONNS, cfg.DB_FSL_MAX_LIFETIME)
	if _, err := fsl.insertAdmin(cfg.FSL_ACCESS_KEY, cfg.FSL_SECRET_KEY); err != nil {
		log.Fatalf("error inserting main user, fatal...: %v", err)
	}
	if fsl.config.FSL_LOCALITY {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("failed to get working directory: %v", err)
		}
		fslite_data_path = wd + "/" + cfg.LOCAL_VOLUMES_DEFAULT_PATH
		if err := os.MkdirAll(fslite_data_path, 0o644); err != nil {
			log.Fatalf("failed to create main volume storage path: %v", err)
		}
	}
	default_volume_cap = min(default_volume_cap, max_volume_cap)
	err := fsl.CreateVolume(ut.Volume{Name: default_volume_name, Path: fslite_data_path + "/" + default_volume_name, Capacity: default_volume_cap})
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			log.Fatalf("failed to create default volume: %v", err)
		}
		log.Print(err)
	}
	jwtValidityHours = cfg.JWT_VALIDITY_HOURS

	return fsl
}

// volume related
func (fsl *FsLite) CreateVolume(v any) error {
	volume, ok1 := v.(ut.Volume)
	if !ok1 {
		return fmt.Errorf("failed to cast to volume")

	}
	if err := volume.Validate(max_volume_cap, default_volume_cap); err != nil {
		return err
	}

	db, err := fsl.dbh.GetConn()
	if err != nil {
		return fmt.Errorf("failed to get the database connection")
	}

	// should check if name exists.
	if _, err = getVolumeByName(db, volume.Name); err == nil { // if err is nil, it exists
		return ut.NewInfo("%s volume already exists", volume.Name)
	} else if err.Error() != "empty" {
		return err
	}

	if fsl.config.FSL_LOCALITY {
		err = os.MkdirAll(fslite_data_path+"/"+volume.Name, 0o644)
		if err != nil {
			return err
		}
	}

	volume.CreationDate = ut.CurrentTime()
	err = insertVolume(db, volume)
	if err != nil {
		os.RemoveAll(fslite_data_path + "/" + volume.Name)
		return err
	}

	return err
}

func (fsl *FsLite) DefaultVolume(local bool) string {
	return default_volume_name
}

func (fsl *FsLite) RemoveVolume(t any) error {
	volume, ok := t.(ut.Volume)
	if !ok {
		return fmt.Errorf("failed to cast to volume")
	}

	if err := volume.Validate(max_volume_cap, default_volume_cap); err != nil {
		return err
	}

	db, err := fsl.dbh.GetConn()
	if err != nil {
		return err
	}
	if volume.Name != "" {
		err = deleteVolumeByName(db, volume.Name)
	} else {
		err = deleteVolume(db, volume.Vid)
	}

	if err == nil && fsl.config.FSL_LOCALITY {
		err = os.RemoveAll(fslite_data_path + "/" + volume.Name)
	}
	return err
}

func (fsl *FsLite) SelectVolumes(how map[string]any) (any, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to get the db conn: %v", err)
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

// resource/uv/gv related
func (fsl *FsLite) Insert(t any) (context.CancelFunc, error) {
	resource, ok := t.(ut.Resource)
	if ok {
		if resource.Vname == "" {
			resource.Vname = default_volume_name
		}

		db, err := fsl.dbh.GetConn()
		if err != nil {
			log.Printf("failed to get the db conn: %v", err)
			return nil, err
		}

		// should check if already exists (in database).
		if _, err = getResourceByName(db, resource.Name); err == nil { // if err is nil, it exists
			return nil, ut.NewInfo("%s object already exists", resource.Name)
		}

		if fsl.config.FSL_LOCALITY {
			outFile, err := os.Create(fslite_data_path + "/" + resource.Vname + "/" + resource.Name)
			if err != nil {
				log.Printf("failed to create a new output file (to save)")
				return nil, err
			}
			defer outFile.Close()

			// Copy from the reader to the file
			_, err = io.Copy(outFile, resource.Reader)
			if err != nil {
				log.Printf("failed to copy to output file")
				return nil, err
			}
		}
		// db
		err = insertResource(db, resource)
		if err != nil {
			log.Printf("failed to insert resources in the db: %v", err)
			return nil, err
		}
		return nil, err
	}
	resources, ok := t.([]ut.Resource)
	if ok {
		db, err := fsl.dbh.GetConn()
		if err != nil {
			log.Printf("failed to get the db conn: %v", err)
			return nil, err
		}
		err = insertResources(db, resources)
		if err != nil {
			log.Printf("failed to insert resources in the db: %v", err)
			return nil, err
		}
		return nil, err
	}

	uv, ok := t.(ut.UserVolume)
	if ok {
		db, err := fsl.dbh.GetConn()
		if err != nil {
			log.Printf("failed to get the db conn: %v", err)
			return nil, err
		}
		err = insertUserVolume(db, uv)
		return nil, err
	}
	uvs, ok := t.([]ut.UserVolume)
	if ok {
		db, err := fsl.dbh.GetConn()
		if err != nil {
			log.Printf("failed to get the db conn: %v", err)
			return nil, err
		}
		err = insertUserVolumes(db, uvs)
		return nil, err
	}

	return nil, fmt.Errorf("failed to cast all types")
}

func (fsl *FsLite) SelectObjects(how map[string]any) (any, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to get the db conn: %v", err)
		return nil, err
	}
	// limit := how["limit"]

	name, ok := how["name"]
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
			return getResourcesByIds(db, rids)
		}
		return nil, err
	}

	return getAllResources(db)
}

func (fsl *FsLite) Stat(t any) (any, error) {
	if fsl.config.FSL_LOCALITY {
		return nil, fmt.Errorf("cannot use stat if locality is turned off")
	}
	resource, ok := t.(ut.Resource)
	if !ok {
		log.Printf("failed to cast to designated struct")
		return nil, fmt.Errorf("failed to cast to designated struct")
	}
	return os.Stat(fslite_data_path + "/" + resource.Vname + "/" + resource.Name)
}

func (fsl *FsLite) Remove(t any) error {
	resource, ok := t.(ut.Resource)
	if !ok {
		log.Printf("failed to cast to designated struct")
		return fmt.Errorf("failed to cast to designated struct")
	}
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve database connection: %v", err)
		return err
	}

	err = deleteResourceByName(db, resource.Name)
	if err != nil {
		return err
	}

	if fsl.config.FSL_LOCALITY {
		err = os.Remove(fslite_data_path + "/" + resource.Vname + "/" + resource.Name)
		if err != nil {
			log.Printf("failed to remove file from local fs")
			return err
		}
	}

	return nil
}

func (fsl *FsLite) Update(t any) error {
	return nil
}

func (fsl *FsLite) Download(t *any) (context.CancelFunc, error) {
	if fsl.config.FSL_LOCALITY {
		return nil, fmt.Errorf("cannot download if locality is off")
	}
	v := *t
	resource, ok := v.(ut.Resource)
	if !ok {
		log.Printf("failed to cast to ut.Resource")
		return nil, fmt.Errorf("failed to cast to ut.Resource")
	}
	resourcePtr := &resource
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve database connection: %v", err)
		return nil, err
	}

	_, err = getResourceByName(db, resourcePtr.Name)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fslite_data_path + "/" + resourcePtr.Vname + "/" + resourcePtr.Name)
	if err != nil {
		return nil, err
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stats: %v", err)
	}
	resourcePtr.Size = stat.Size()
	resourcePtr.Reader = file

	*t = *resourcePtr

	return nil, nil
}

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
		log.Printf("failed to retrieve database connection: %v", err)
		return err
	}

	if fsl.config.FSL_LOCALITY {
		sr, err := os.Open(fslite_data_path + "/" + src.Vname + "/" + src.Name)
		if err != nil {
			log.Printf("failed to read the src file")
			return err
		}
		defer sr.Close()
		sr1, err := io.ReadAll(sr)
		if err != nil {
			log.Printf("failed to read the src file to a buffer")
			return err
		}

		ds, err := os.OpenFile(fslite_data_path+"/"+dst.Vname+"/"+dst.Name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			log.Printf("failed to open the dst file")
			return err
		}
		defer ds.Close()

		_, err = ds.Write(sr1)
		if err != nil {
			log.Printf("failed to write to output file")
		}
	}

	// update db
	err = insertResource(db, dst)
	if err != nil {
		log.Printf("failed to insert to the db0")
	}
	return err
}

func (fsl *FsLite) Share(method string, t any) (any, error) {
	return nil, nil
}

func (fsl *FsLite) claimVolumeSpace(size int64, volumeName, uid string) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve database connection: %v", err)
		return err
	}

	volume, err := getVolumeByName(db, volumeName)
	if err != nil {
		return err
	}
	size_inGB := ut.SizeInGb(size)
	// check for current volume usage.
	new_usage_inGB := volume.Usage + size_inGB
	if new_usage_inGB > volume.Capacity {
		log.Printf("volume is full.")
		return fmt.Errorf("claim exceeds capacity")
	}

	// if not dynamic, we should check for per user/group quota
	iuid, err := strconv.Atoi(uid)
	if err != nil {
		return err
	}
	// if it doesn't exist, create it
	uv, err := getUserVolumeByUid(db, iuid)
	if err != nil {
		err = insertUserVolume(db, ut.UserVolume{Updated_at: ut.CurrentTime(), Vid: volume.Vid, Uid: iuid, Usage: size_inGB})
		if err != nil {
			log.Printf("failed to insert uv ")
			return err
		}
	}

	// update all usages
	// volume
	// volume claims user/group
	uv.Usage += size_inGB
	volume.Usage = new_usage_inGB

	err = updateVolume(db, volume)
	if err != nil {
		log.Printf("failed to update volume usages: %v", err)
		return err
	}
	err = updateUserVolume(db, uv)
	if err != nil {
		log.Printf("failed to update user volume usages: %v", err)
		return err
	}

	return nil
}

func (fsl *FsLite) releaseVolumeSpace(size int64, volumeName, uid string) error {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve database connection: %v", err)
		return err
	}

	volume, err := getVolumeByName(db, volumeName)
	if err != nil {
		log.Printf("could not retrieve volume: %v", err)
		return fmt.Errorf("could not retrieve volume: %w", err)
	}

	size_inGB := ut.SizeInGb(size)
	new_usage_inGB := max(volume.Usage-size_inGB, 0)

	iuid, err := strconv.Atoi(uid)
	if err != nil {
		return err
	}
	uv, err := getUserVolumeByUid(db, iuid)
	if err != nil {
		return err
	}

	// update all usages
	// volume
	// volume claims user/group
	uv.Usage = max(0, uv.Usage-size_inGB)
	volume.Usage = new_usage_inGB

	err = updateVolume(db, volume)
	if err != nil {
		log.Printf("failed to update volume usages: %v", err)
		return err
	}
	err = updateUserVolume(db, uv)
	if err != nil {
		log.Printf("failed to update user volume usages: %v", err)
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
		return getUserVolumesByVolumeIds(db, strings.Split(vids, ","))

	} else if ok2 && uids != "" {
		// log.Printf("selecting uvs by uids")
		return getUserVolumesByUserIds(db, strings.Split(uids, ","))

	} else {
		// log.Printf("selecting all uvs")
		return getAllUserVolumes(db)
	}

}

/* playground functions... working on it... i don't like them */
func (fsl *FsLite) sel(sel, table, by, byvalue string, limit int) ([]any, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		// log.Printf("failed to get the db conn: %v", err)
		return nil, err
	}
	results, err := get(db, sel, table, by, byvalue, limit, pickScanFn(table))
	if err != nil {
		// log.Prntf("failed to get the desired data: %v", err)
		return nil, err
	}
	return results, nil

}
func (fsl *FsLite) selectOne(sel, table, by, byvalue string) (any, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		// log.Printf("failed to get the db conn: %v", err)
		return nil, err
	}
	switch table {
	case "resources":
		result, err := getResource(db, sel, table, by, byvalue)
		if err != nil {
			// log.Printf("failed to get the desired data: %v", err)
			return nil, err
		}
		return result, nil
	case "volumes":
		result, err := getVolume(db, sel, table, by, byvalue)
		if err != nil {
			// log.Printf("failed to get the desired data: %v", err)
			return nil, err
		}
		return result, nil
	case "userVolume":
		result, err := getUserVolume(db, sel, table, by, byvalue)
		if err != nil {
			// log.Printf("failed to get the desired data: %v", err)
			return nil, err
		}
		return result, nil

	default:
		return nil, ut.NewError("invalid table name, not supported: %s", table)
	}

}
