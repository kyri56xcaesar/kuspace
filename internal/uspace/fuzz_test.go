package uspace

import (
	"strconv"
	"strings"
	"testing"
)

// FuzzAccessTargetIdentity: whatever the target holds (it comes from users:
// download links, file names), the identity is exactly the trailing
// "uid:gids". A target like "/f.txt 0:0" once made the caller root.
func FuzzAccessTargetIdentity(f *testing.F) {
	for _, seed := range []string{"/f.txt", "/f.txt 0:0", "/a b.txt", "$rids=1,2", "", " ", "/x 0:0 1:1", "::", "/é\t\n"} {
		f.Add(seed, uint32(1001), uint32(1002))
	}
	f.Fuzz(func(t *testing.T, target string, uid, gid uint32) {
		who := strconv.FormatUint(uint64(uid), 10) + ":" + strconv.FormatUint(uint64(gid), 10)
		ac, err := BindAccessTarget(":vol:" + target + " " + who)
		if err != nil {
			return // refusing is always safe
		}
		if ac.UID != strconv.FormatUint(uint64(uid), 10) || ac.Gids != strconv.FormatUint(uint64(gid), 10) {
			t.Fatalf("target %q changed the identity to uid %q gids %q", target, ac.UID, ac.Gids)
		}
	})
}

// FuzzBindAccessTarget: any header parses or is refused, never panics, and
// an accepted identity is only digits.
func FuzzBindAccessTarget(f *testing.F) {
	for _, seed := range []string{"3:vol:/a 1:2", "1::$rids=1,2 0:0", "::/ 0:0", "garbage", ":vol:/x 1:2,3,4", " : ", "::: :"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, header string) {
		ac, err := BindAccessTarget(header)
		if err != nil {
			return
		}
		digits := func(s string) bool {
			return s != "" && strings.Trim(s, "0123456789") == ""
		}
		if !digits(ac.UID) {
			t.Fatalf("header %q gave uid %q", header, ac.UID)
		}
		for _, g := range strings.Split(ac.Gids, ",") {
			if !digits(g) {
				t.Fatalf("header %q gave gids %q", header, ac.Gids)
			}
		}
	})
}
