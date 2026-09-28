package fslite

import (
	"path/filepath"
	"strings"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

// FuzzNormalizeName: one stored form - a single leading "/" - reached in
// one step.
func FuzzNormalizeName(f *testing.F) {
	for _, s := range []string{"a.txt", "/a.txt", "//a", "", "/", "a/b/", "///"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		n := NormalizeName(name)
		if !strings.HasPrefix(n, "/") || strings.HasPrefix(n, "//") {
			t.Fatalf("NormalizeName(%q) = %q", name, n)
		}
		if NormalizeName(n) != n {
			t.Fatalf("not idempotent: %q -> %q -> %q", name, n, NormalizeName(n))
		}
	})
}

// FuzzObjectPathStaysInside: no volume or object name reaches outside the
// data directory.
func FuzzObjectPathStaysInside(f *testing.F) {
	for _, s := range [][2]string{{"vol", "a.txt"}, {"..", "x"}, {"vol", "../../etc/passwd"}, {"a/../../b", "c"}, {"", ""}, {"/", "/"}, {"vol", "..\\x"}} {
		f.Add(s[0], s[1])
	}
	root := "/srv/fsl"
	fsl := &FsLite{dataPath: root}
	f.Fuzz(func(t *testing.T, volume, name string) {
		p := fsl.objectPath(volume, name)
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == "." {
			t.Fatalf("objectPath(%q, %q) = %q escapes %s", volume, name, p, root)
		}
	})
}

// FuzzPrefixListing: a prefix lists exactly the names that start with it,
// whatever LIKE's wildcards ("%", "_", "\") it contains.
func FuzzPrefixListing(f *testing.F) {
	fsl := newTestFsl(f)
	names := []string{"/a_b", "/aXb", "/a%b", "/a%", `/a\b`, "/ab", "/b", "/a b"}
	vid := mustVolume(f, fsl, "vol1", 0)
	for _, n := range names {
		mustResource(f, fsl, ut.Resource{Name: n, Vname: "vol1", VID: vid, UID: 1, GID: 1, Type: "file"})
	}
	for _, s := range []string{"a_", "a%", `a\`, "a", "", "%", "_", `\`, "a b"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, prefix string) {
		res, err := fsl.SelectObjects(ctx, map[string]any{"vname": "vol1", "prefix": prefix})
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for _, r := range res.([]ut.Resource) {
			got[r.Name] = true
		}
		want := NormalizeName(prefix)
		for _, n := range names {
			if strings.HasPrefix(n, want) != got[n] {
				t.Fatalf("prefix %q: %q listed=%v", prefix, n, got[n])
			}
		}
	})
}
