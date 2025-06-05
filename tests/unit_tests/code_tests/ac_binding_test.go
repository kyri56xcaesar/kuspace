package test

import (
	"testing"

	u "kyri56xcaesar/kuspace/internal/uspace"
	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/zeebo/assert"
)

func TestBinding(t *testing.T) {
	testH := "3:test-volume:/hello/motf4k 0:0,100,1000"

	ac, err := u.BindAccessTarget(testH)
	if err != nil {
		panic(err)
	}

	assert.DeepEqual(t, ac, ut.AccessClaim{
		UID:        "0",
		Gids:       "0,100,1000",
		Vid:        "3",
		Vname:      "test-volume",
		Target:     "/hello/motf4k",
		HasKeyword: false,
	})

	testH = "1::$rids=1,2,3 0:0"
	ac, err = u.BindAccessTarget(testH)
	if err != nil {
		panic(err)
	}
	assert.DeepEqual(t, ac, ut.AccessClaim{
		UID:        "0",
		Gids:       "0",
		Vid:        "1",
		Vname:      "",
		Target:     "1,2,3",
		HasKeyword: true,
	})

	testH = "1::$rids=1,2,3 0:0"
	_, err = u.BindAccessTarget(testH)

	assert.NoError(t, err)
	// assert.DeepEqual(t, ac, ut.AccessClaim{
	// 	UID:        "0",
	// 	Gids:       "0",
	// 	Vid:        "",
	// 	Vname:      "sas",
	// 	Target:     "1,2,3",
	// 	HasKeyword: true,
	// })
}

func TestBindingFalse(t *testing.T) {
	testH := "::$rids=1,2,3 0:0"
	_, err := u.BindAccessTarget(testH)
	assert.Error(t, err)
	// assert.DeepEqual(t, ac, ut.AccessClaim{
	// 	UID:        "0",
	// 	Gids:       "0",
	// 	Vid:        "",
	// 	Vname:      "",
	// 	Target:     "1,2,3",
	// 	HasKeyword: true,
	// })
}
