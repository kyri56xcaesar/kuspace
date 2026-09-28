package authn

import (
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/golang-jwt/jwt/v5"
)

// FuzzVerify: any string is accepted only if it is exactly a token signed
// with the key; the verifier never panics.
func FuzzVerify(f *testing.F) {
	good := token(&testing.T{}, testJWTKey, jwt.SigningMethodHS256, nil)
	f.Add(good)
	f.Add(good + "x")
	f.Add("not.a.jwt")
	f.Add("")
	f.Add("eyJhbGciOiJub25lIn0.e30.")
	v := NewVerifier(ut.EnvConfig{TokensConfig: ut.TokensConfig{JwtSecretKey: testJWTKey}})
	f.Fuzz(func(t *testing.T, raw string) {
		c, err := v.Verify(raw)
		if err != nil {
			return
		}
		// accepted: it must re-verify under the key and carry a user
		if c.UserID == "" {
			t.Fatalf("accepted %q without a user", raw)
		}
		if _, err := jwt.ParseWithClaims(raw, &Claims{}, func(*jwt.Token) (any, error) { return testJWTKey, nil },
			jwt.WithValidMethods([]string{"HS256"})); err != nil {
			t.Fatalf("accepted %q that the key doesn't sign: %v", raw, err)
		}
	})
}
