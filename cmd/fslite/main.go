package main

import (
	"kyri56xcaesar/myThesis/internal/utils"
	"kyri56xcaesar/myThesis/pkg/fslite"
)

func main() {
	fslite := fslite.NewFsLite(utils.LoadConfig("configs/fslite.conf"))
	fslite.ListenAndServe()
}
