package utils

import "testing"

// file owned by uid 1001, group 2000
func res(perms string) Resource { return Resource{UID: 1001, GID: 2000, Perms: perms} }

var (
	owner  = AccessClaim{UID: "1001", Gids: "1000,1002"}
	member = AccessClaim{UID: "1005", Gids: "1000,2000"} // in the file's group
	other  = AccessClaim{UID: "1007", Gids: "1000,1008"}
)

func TestPermissionChecks(t *testing.T) {
	cases := []struct {
		perms              string
		who                string
		ac                 AccessClaim
		read, write, owner bool
	}{
		{"rw-r--r--", "owner", owner, true, true, true},
		{"rw-r--r--", "member", member, true, false, false},
		{"rw-r--r--", "other", other, true, false, false},
		{"rw-------", "owner", owner, true, true, true},
		{"rw-------", "member", member, false, false, false},
		{"rw-------", "other", other, false, false, false},
		{"rw-rw----", "member", member, true, true, false}, // group can write, still not owner
		{"rw-rw----", "other", other, false, false, false},
		{"---rw-rw-", "owner", owner, false, false, true}, // owner class applies to the owner, like Unix
		{"bogus", "owner", owner, false, false, true},     // unparsable perms deny access
	}
	for _, c := range cases {
		r := res(c.perms)
		if got := r.HasAccess(c.ac); got != c.read {
			t.Errorf("%s %s: read = %v, want %v", c.perms, c.who, got, c.read)
		}
		if got := r.HasWriteAccess(c.ac); got != c.write {
			t.Errorf("%s %s: write = %v, want %v", c.perms, c.who, got, c.write)
		}
		if got := r.IsOwner(c.ac); got != c.owner {
			t.Errorf("%s %s: owner = %v, want %v", c.perms, c.who, got, c.owner)
		}
	}
}
