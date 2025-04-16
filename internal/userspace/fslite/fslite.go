package fslite

import (
	ut "kyri56xcaesar/myThesis/internal/utils"
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

// need to implement the interface StorageSystem
func (fsl *FsLite) Insert(t []any) error {
	return nil
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
func (fsl *FsLite) Update(t any) error {
	return nil
}
func (fsl *FsLite) Remove(t any) error {
	return nil
}
