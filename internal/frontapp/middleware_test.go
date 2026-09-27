package frontendapp

import "testing"

func TestPrimaryFirst(t *testing.T) {
	cases := []struct{ gids, pgroup, want string }{
		{"1000,1012", "1012", "1012,1000"},
		{"1012,1000", "1012", "1012,1000"},
		{"1000", "1012", "1012,1000"}, // primary missing from the list is added
		{"1000,1012", "", "1000,1012"},
		{"0,1000", "0", "0,1000"}, // no real primary group
	}
	for _, c := range cases {
		if got := primaryFirst(c.gids, c.pgroup); got != c.want {
			t.Errorf("primaryFirst(%q, %q) = %q, want %q", c.gids, c.pgroup, got, c.want)
		}
	}
}
