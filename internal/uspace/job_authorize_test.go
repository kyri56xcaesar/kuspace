package uspace

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
)

// TestAuthorizeJobIO: jobs run with service credentials, so this check is
// all that stops a job from reading or overwriting someone else's file.
func TestAuthorizeJobIO(t *testing.T) {
	srv := newTestService(t)
	vid := srv.volumeID(t.Context(), "vol1")
	for _, name := range []string{"in.csv", "out.csv"} {
		now := ut.CurrentTime()
		r := ut.Resource{Name: name, Vname: "vol1", VID: vid, UID: 1001, GID: 1002, Type: "file",
			Perms: ut.DefaultFilePerms, CreatedAt: now, UpdatedAt: now, AccessedAt: now} // rw-r-----
		if err := srv.fsl.Insert(t.Context(), r); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name, target  string // Access-Target "who" part ("" = no header)
		uid           int64  // job owner
		input, output string
		allowed       bool
	}{
		{"owner, new output", "1001:1001,1002", 1001, "vol1/in.csv", "vol1/new.csv", true},
		{"owner overwrites own output", "1001:1001", 1001, "vol1/in.csv", "vol1/out.csv", true},
		{"root", "0:0", 5, "vol1/in.csv", "vol1/out.csv", true},
		{"group member reads input", "3003:3003,1002", 3003, "vol1/in.csv", "vol1/new.csv", true},
		{"group member can't overwrite (r only)", "3003:3003,1002", 3003, "vol1/in.csv", "vol1/out.csv", false},
		{"other user's private input", "4004:4004", 4004, "vol1/in.csv", "vol1/new.csv", false},
		{"missing input looks the same as private", "4004:4004", 4004, "vol1/nothing.csv", "vol1/new.csv", false},
		{"job for someone else", "4004:4004", 1001, "vol1/in.csv", "vol1/new.csv", false},
		{"no identity", "", 1001, "vol1/in.csv", "vol1/new.csv", false},
		{"input without a volume", "1001:1001", 1001, "in.csv", "vol1/new.csv", false},
		{"output without a name", "1001:1001", 1001, "vol1/in.csv", "vol1/", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/job", nil)
			if tc.target != "" {
				c.Request.Header.Set("Access-Target", "0:vol1:/ "+tc.target)
			}
			err := srv.authorizeJobIO(c, ut.Job{UID: tc.uid, Input: tc.input, Output: tc.output})
			switch {
			case tc.allowed && err != nil:
				t.Errorf("refused: %v", err)
			case !tc.allowed && err == nil:
				t.Error("allowed")
			case !tc.allowed && !errors.Is(err, errJobForbidden):
				t.Errorf("refused with a server error instead of 403: %v", err)
			}
		})
	}
}
