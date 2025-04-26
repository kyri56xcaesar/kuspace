package fslite

import (
	"context"
	"fmt"
	ut "kyri56xcaesar/myThesis/internal/utils"
	"log"
	"os"
	"strings"
)

var (
	default_volume_name string = "default_ku_space_volume"
)

type FsLite struct {
	dbh ut.DBHandler
}

func NewFsLite(cfg ut.EnvConfig) FsLite {
	return FsLite{
		dbh: ut.NewDBHandler(cfg.DB_RV, cfg.DB_RV_PATH, cfg.DB_RV_DRIVER),
	}
}

func (fsl *FsLite) CreateVolume(volumeId any) error {
	return nil
}

// need to implement the interface StorageSystem
func (fsl *FsLite) Insert(t any) (context.CancelFunc, error) {
	return nil, nil
}

func (fsl *FsLite) SelectVolumes(how map[string]any) ([]any, error) {
	return nil, nil
}

func (fsl *FsLite) SelectObjects(how map[string]any) ([]any, error) {
	return nil, nil
}

func (fsl *FsLite) Select(sel, table, by, byvalue string, limit int) ([]any, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		// log.Printf("failed to get the db conn: %v", err)
		return nil, err
	}
	results, err := Get(db, sel, table, by, byvalue, limit, PickScanFn(table))
	if err != nil {
		// log.Prntf("failed to get the desired data: %v", err)
		return nil, err
	}
	return results, nil

}
func (fsl *FsLite) SelectOne(sel, table, by, byvalue string) (any, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		// log.Printf("failed to get the db conn: %v", err)
		return nil, err
	}
	switch table {
	case "resources":
		result, err := GetResource(db, sel, table, by, byvalue)
		if err != nil {
			// log.Printf("failed to get the desired data: %v", err)
			return nil, err
		}
		return result, nil
	case "volumes":
		result, err := GetVolume(db, sel, table, by, byvalue)
		if err != nil {
			// log.Printf("failed to get the desired data: %v", err)
			return nil, err
		}
		return result, nil
	case "userVolume":
		result, err := GetUserVolume(db, sel, table, by, byvalue)
		if err != nil {
			// log.Printf("failed to get the desired data: %v", err)
			return nil, err
		}
		return result, nil
	case "groupVolume":
		result, err := GetGroupVolume(db, sel, table, by, byvalue)
		if err != nil {
			// log.Printf("failed to get the desired data: %v", err)
			return nil, err
		}
		return result, nil
	default:
		return nil, ut.NewError("invalid table name, not supported: %s", table)
	}

}
func (fsl *FsLite) Stat(t any, locally bool) (any, error) {
	return nil, nil
}
func (fsl *FsLite) Remove(t any) error {
	return nil
}

func (fsl *FsLite) RemoveVolume(t any) error {
	return nil
}

func (fsl *FsLite) Update(t any) error {
	return nil
}

func (fsl *FsLite) Download(t *any) (context.CancelFunc, error) {
	return nil, nil
}

func (fsl *FsLite) Copy(s, d any) error {
	return nil
}

func (fsl *FsLite) ClaimVolumeSpace(size int64, ac ut.AccessClaim) error {
	// for now:
	// ac.Vid = 1

	res, err := fsl.SelectOne("", "volumes", "vid", "1")
	if err != nil {
		log.Printf("could not retrieve volume: %v", err)
		return fmt.Errorf("could not retrieve volume: %w", err)
	}
	volume := res.(ut.Volume)
	// check for current volume usage.
	// size is in Bytes
	size_inGB := float64(size) / 1000000000
	new_usage_inGB := volume.Usage + size_inGB

	if new_usage_inGB > volume.Capacity {
		log.Printf("volume is full.")
		return fmt.Errorf("claim exceeds capacity")
	}

	// if not dynamic, we should check for per user/group quota
	res, err = fsl.SelectOne("", "userVolume", "uid", ac.Uid)
	if err != nil {
		log.Printf("failed to retrieve user volume: %v", err)
		return err
	}
	uv := res.(ut.UserVolume)

	res, err = fsl.Select("", "groupVolume", "gid", ac.Gids, 0)
	if err != nil {
		log.Printf("failed to retrieve group volume: %v", err)
		return err
	}
	gvs := res.([]ut.GroupVolume)

	// update all usages
	// volume
	// volume claims user/group
	uv.Usage += size_inGB
	for index, gv := range gvs {
		log.Printf("gv: %+v", gv)
		gvs[index].Usage += size_inGB
	}
	volume.Usage = new_usage_inGB

	log.Printf("updated volume: %+v", volume)
	log.Printf("updated uv: %+v", uv)
	log.Printf("updated gvs: %+v", gvs)

	// err = srv.storage.UpdateVolume(volume)
	// if err != nil {
	// 	log.Printf("failed to update volume usages: %v", err)
	// 	return err
	// }
	// err = srv.storage.UpdateUserVolume(uv)
	// if err != nil {
	// 	log.Printf("failed to update user volume usages: %v", err)
	// 	return err
	// }
	// err = srv.storage.UpdateGroupVolumes(gvs_casted)
	// if err != nil {
	// 	log.Printf("failed to update group volume usages: %v", err)
	// 	return err
	// }

	return nil
}

func (fsl *FsLite) ReleaseVolumeSpace(size int64, ac ut.AccessClaim) error {

	res, err := fsl.SelectOne("", "volumes", "vid", "1")
	if err != nil {
		log.Printf("could not retrieve volume: %v", err)
		return fmt.Errorf("could not retrieve volume: %w", err)
	}
	volume := res.(ut.Volume)

	size_inGB := float64(size) / 1000000000
	new_usage_inGB := volume.Usage - size_inGB

	if new_usage_inGB < 0 {
		new_usage_inGB = 0
	}

	res, err = fsl.SelectOne("", "userVolume", "uid", ac.Uid)
	if err != nil {
		log.Printf("failed to retrieve user volume: %v", err)
		return err
	}
	uv := res.(ut.UserVolume)

	res, err = fsl.SelectOne("", "groupVolume", "gid", strings.Split(strings.TrimSpace(ac.Gids), ",")[0])
	if err != nil {
		log.Printf("failed to retrieve group volume: %v", err)
		return err
	}
	gv := res.(ut.GroupVolume)

	// update all usages
	// volume
	// volume claims user/group
	uv.Usage -= size_inGB
	gv.Usage -= size_inGB
	volume.Usage = new_usage_inGB

	// err = srv.storage.UpdateVolume(volume)
	// if err != nil {
	// 	log.Printf("failed to update volume usages: %v", err)
	// 	return err
	// }
	// err = srv.storage.UpdateUserVolume(uv)
	// if err != nil {
	// 	log.Printf("failed to update user volume usages: %v", err)
	// 	return err
	// }
	// err = srv.storage.UpdateGroupVolume(gv)
	// if err != nil {
	// 	log.Printf("failed to update group volume usages: %v", err)
	// 	return err
	// }

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

func (fsl *FsLite) DefaultVolume(local bool) string {
	return default_volume_name
}
