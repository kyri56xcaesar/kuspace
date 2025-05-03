package main

import (
	"kyri56xcaesar/myThesis/internal/userspace/fslite"
	"kyri56xcaesar/myThesis/internal/utils"
)

func main() {
	fslite := fslite.NewFsLite(utils.LoadConfig("internal/userspace/fslite/main/fslite.conf"))
	fslite.ListenAndServe()
}
