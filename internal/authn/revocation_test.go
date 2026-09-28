package authn

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/golang-jwt/jwt/v5"
)

// fakeIntrospection answers minioth's GET /v1/user/token: 400 for revoked
// tokens, 200 otherwise, counting calls.
func fakeIntrospection(t *testing.T, revoked *atomic.Value, calls *atomic.Int32) (string, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/user/token" {
			http.NotFound(w, r)

			return
		}
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if bad, _ := revoked.Load().(string); bad != "" && raw == bad {
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	host, p, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))

	return host, p
}

func verifierFor(addr, port string) *Verifier {
	return NewVerifier(ut.EnvConfig{
		TokensConfig: ut.TokensConfig{JwtSecretKey: testJWTKey, JwtRevocationCheck: true},
		PeersConfig:  ut.PeersConfig{AuthAddress: addr, AuthPort: port},
	})
}

func TestRevokedTokensAreRejected(t *testing.T) {
	var revoked atomic.Value
	var calls atomic.Int32
	addr, port := fakeIntrospection(t, &revoked, &calls)
	v := verifierFor(addr, port)
	tok := token(t, testJWTKey, jwt.SigningMethodHS256, nil)

	for range 5 {
		if _, err := v.Verify(tok); err != nil {
			t.Fatalf("valid token: %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("minioth asked %d times for one token within the cache window, want 1", calls.Load())
	}

	// minioth revokes it (logout): the cached "valid" hides that until it
	// expires, unless the service that revoked it tells its verifier
	revoked.Store(tok)
	if _, err := v.Verify(tok); err != nil {
		t.Fatalf("within the cache window: %v", err)
	}
	v.Forget(tok)
	if _, err := v.Verify(tok); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked token: %v, want ErrRevoked", err)
	}
	before := calls.Load()
	if _, err := v.Verify(tok); !errors.Is(err, ErrRevoked) || calls.Load() != before {
		t.Errorf("a revoked token must stay rejected without asking again (%v, %d calls)", err, calls.Load()-before)
	}
	// a forged token never reaches minioth: the signature fails first
	forged := token(t, []byte("other-key"), jwt.SigningMethodHS256, nil)
	if _, err := v.Verify(forged); err == nil || calls.Load() != before {
		t.Errorf("forged token: %v, %d calls", err, calls.Load()-before)
	}
}

func TestRevocationCheckFailsOpenWhenMiniothIsDown(t *testing.T) {
	v := verifierFor("127.0.0.1", "1") // nothing listens there
	if _, err := v.Verify(token(t, testJWTKey, jwt.SigningMethodHS256, nil)); err != nil {
		t.Errorf("minioth down: %v (signature-valid tokens stay accepted)", err)
	}
}
