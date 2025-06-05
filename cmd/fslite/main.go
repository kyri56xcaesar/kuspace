// Package main includes the main definition
package main

import (
	"kyri56xcaesar/kuspace/internal/utils"
	"kyri56xcaesar/kuspace/pkg/fslite"
)

func main() {
	fslite := fslite.NewFsLite(utils.LoadConfig("configs/fslite.conf"))
	fslite.ListenAndServe()
}
