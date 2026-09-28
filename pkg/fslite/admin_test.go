package fslite

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAdminTokens(t *testing.T) {
	if _, err := (tokenSigner{validity: time.Hour}).issue("id", "admin"); err == nil {
		t.Fatal("issued a token without a configured key")
	}

	ts := tokenSigner{key: deriveTokenKey([]byte("configured-secret")), validity: time.Hour}
	tok, err := ts.issue("id-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if c, err := ts.verify(tok); err != nil || c.Username != "admin" {
		t.Fatalf("own token rejected: %v", err)
	}

	claims := CustomClaims{ID: "id-0", Username: "attacker", RegisteredClaims: jwt.RegisteredClaims{
		Issuer: "fslite", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	for name, key := range map[string][]byte{
		"the old hardcoded key": []byte("r4nd0m"),
		"the raw JWT secret":    []byte("configured-secret"), // fslite's key is derived, never the raw one
	} {
		forged, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
		if _, err := ts.verify(forged); err == nil {
			t.Errorf("token signed with %s accepted", name)
		}
	}
	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(ts.key)
	if _, err := ts.verify(hs512); err == nil {
		t.Error("HS512 token accepted (only HS256 is issued)")
	}
	expired, _ := (tokenSigner{key: ts.key, validity: -time.Minute}).issue("id-1", "admin")
	if _, err := ts.verify(expired); err == nil {
		t.Error("expired token accepted")
	}
}

func TestAdminPasswordRules(t *testing.T) {
	for pw, ok := range map[string]bool{
		"short1":                  false,
		"with-dash-and.dot":       true, // used to be rejected
		"has space in it":         false,
		string(make([]byte, 73)):  false, // beyond bcrypt's limit (and not printable)
		"Aa1!Aa1!Aa1!Aa1!Aa1!xyz": true,
	} {
		err := (&Admin{Username: "admin", Password: pw}).validate()
		if (err == nil) != ok {
			t.Errorf("password %q: err=%v, want ok=%v", pw, err, ok)
		}
	}
}

func TestTokenValidity(t *testing.T) {
	for hours, want := range map[float64]time.Duration{0.5: 30 * time.Minute, 2: 2 * time.Hour, 0: 4 * time.Hour} {
		if got := hoursToDuration(hours); got != want {
			t.Errorf("%v hours -> %v, want %v", hours, got, want)
		}
	}
}

// Two instances in one process keep their own settings: the data path and
// token key used to be package variables that the last NewFsLite overwrote.
func TestInstancesAreIndependent(t *testing.T) {
	a, b := newTestFsl(t), newTestFsl(t)
	withLocality(t, a, "volume1")
	withLocality(t, b, "volume1")
	if a.dataPath == b.dataPath {
		t.Fatal("instances share a data path")
	}
	a.tokens.key = []byte("key-a")
	b.tokens.key = []byte("key-b")
	tok, _ := a.tokens.issue("1", "admin")
	if _, err := a.tokens.verify(tok); err != nil {
		t.Errorf("a rejects its own token: %v", err)
	}
	if _, err := b.tokens.verify(tok); err == nil {
		t.Error("b accepts a's token")
	}
}

func TestObjectPathStaysInside(t *testing.T) {
	fsl := &FsLite{dataPath: "/srv/fsl"}
	cases := map[[2]string]string{
		{"vol", "a.txt"}:          "/srv/fsl/vol/a.txt",
		{"vol", ""}:               "/srv/fsl/vol",
		{"..", "x"}:               "/srv/fsl/_/x",
		{"vol", "../../etc/pass"}: "/srv/fsl/vol/pass",
		{"a/../../b", "c"}:        "/srv/fsl/b/c",
	}
	for in, want := range cases {
		if got := fsl.objectPath(in[0], in[1]); got != want {
			t.Errorf("objectPath(%q, %q) = %s, want %s", in[0], in[1], got, want)
		}
	}
}
