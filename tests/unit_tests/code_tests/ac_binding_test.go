package test

import (
	"testing"

	u "kyri56xcaesar/myThesis/internal/userspace"
	ut "kyri56xcaesar/myThesis/internal/utils"

	"github.com/zeebo/assert"
)

func TestBinding(t *testing.T) {
	test_h := "3:test-volume:/hello/motf4k 0:0,100,1000"

	ac, err := u.BindAccessTarget(test_h)
	if err != nil {
		panic(err)
	}

	assert.DeepEqual(t, ac, ut.AccessClaim{
		Uid:        "0",
		Gids:       "0,100,1000",
		Vid:        "3",
		Vname:      "test-volume",
		Target:     "/hello/motf4k",
		HasKeyword: false,
	})

	test_h = "1::$rids=1,2,3 0:0"
	ac, err = u.BindAccessTarget(test_h)
	if err != nil {
		panic(err)
	}
	assert.DeepEqual(t, ac, ut.AccessClaim{
		Uid:        "0",
		Gids:       "0",
		Vid:        "1",
		Vname:      "",
		Target:     "1,2,3",
		HasKeyword: true,
	})

	test_h = "1::$rids=1,2,3 0:0"
	_, err = u.BindAccessTarget(test_h)

	assert.NoError(t, err)
	// assert.DeepEqual(t, ac, ut.AccessClaim{
	// 	Uid:        "0",
	// 	Gids:       "0",
	// 	Vid:        "",
	// 	Vname:      "sas",
	// 	Target:     "1,2,3",
	// 	HasKeyword: true,
	// })
}

func TestBindingFalse(t *testing.T) {
	test_h := "::$rids=1,2,3 0:0"
	_, err := u.BindAccessTarget(test_h)
	assert.Error(t, err)
	// assert.DeepEqual(t, ac, ut.AccessClaim{
	// 	Uid:        "0",
	// 	Gids:       "0",
	// 	Vid:        "",
	// 	Vname:      "",
	// 	Target:     "1,2,3",
	// 	HasKeyword: true,
	// })
}
