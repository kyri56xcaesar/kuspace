package jobs

import (
	"sort"

	"kyri56xcaesar/kuspace/internal/model"
)

// LanguageApps lists the code languages (codeModes) as catalogue entries.
func LanguageApps() []model.Application {
	names := make([]string, 0, len(codeModes))
	for n := range codeModes {
		names = append(names, n)
	}
	sort.Strings(names)
	apps := make([]model.Application, 0, len(names))
	for _, n := range names {
		m := codeModes[n]
		apps = append(apps, model.Application{
			Name: n, Image: m.image + ":" + m.tag, Version: m.tag, Author: "kuspace", Status: "available",
			Description: m.desc + ": write it in the editor. The job's input and output are presigned URLs in INPUT_URL and OUTPUT_URL.",
		})
	}

	return apps
}
