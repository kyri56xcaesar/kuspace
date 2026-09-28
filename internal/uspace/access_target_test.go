package uspace_test

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
		VID:        "3",
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
		VID:        "1",
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

// A target is user input (download links, file names); it must never be
// able to supply the identity. The parser used to split at the first space,
// so "/f.txt 0:0" made the caller root.
func TestTargetCannotSmuggleIdentity(t *testing.T) {
	ac, err := u.BindAccessTarget(":vol:/victim.txt 0:0 1001:1002")
	if err != nil {
		t.Fatal(err)
	}
	if ac.UID != "1001" || ac.Gids != "1002" || ac.Target != "/victim.txt 0:0" {
		t.Errorf("parsed %+v: want uid 1001 and the whole target", ac)
	}

	ac, err = u.BindAccessTarget(":vol:/my report.pdf 1001:1002")
	if err != nil || ac.Target != "/my report.pdf" || ac.UID != "1001" {
		t.Errorf("file name with a space: %+v, %v", ac, err)
	}

	for _, bad := range []string{
		":vol:/f.txt 0:0 x",
		":vol:/f.txt root:0",
		":vol:/f.txt 1001:",
		":vol:/f.txt 1001:1002,",
		":vol:/f.txt :1002",
		":vol:/f.txt 1001:10 02",
		":vol:/f.txt",
	} {
		if ac, err := u.BindAccessTarget(bad); err == nil {
			t.Errorf("%q accepted as %+v", bad, ac)
		}
	}
}
