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
func (fsl *FsLite) Insert(t any) error {
	return nil
}
func (fsl *FsLite) Select(t any) ([]any, error) {
	return nil, nil
}
func (fsl *FsLite) SelectOne(t any) (any, error) {
	return nil, nil
}
func (fsl *FsLite) Update(t any) error {
	return nil
}
func (fsl *FsLite) Remove(t any) error {
	return nil
}
