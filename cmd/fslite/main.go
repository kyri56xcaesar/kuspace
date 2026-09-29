// Package main includes the main definition
package main

import (
	"kyri56xcaesar/kuspace/internal/config"
	"kyri56xcaesar/kuspace/pkg/fslite"
)

func main() {
	fslite := fslite.NewFsLite(config.LoadConfig("configs/fslite.conf", config.Service, config.Auth, config.Tokens, config.Storage, config.Fslite))
	fslite.ListenAndServe()
}
