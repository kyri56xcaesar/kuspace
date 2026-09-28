package authn

import (
	"context"
	"crypto/sha256"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"
)

/*
	Revocation (minioth v1.1.0+): logout, a password change or an admin can
	revoke a user's tokens before they expire. Signatures can't show that,
	so after the local check the verifier asks minioth's introspection
	(GET /v1/user/token), which rejects revoked tokens:

	- answers are cached per token: a valid one for revocationTTL (so a
	  revocation takes effect within that time without a request to minioth
	  per call), a revoked one until the token would have expired;
	- if minioth can't be reached the token is accepted on its signature
	  alone and the outage logged (revocation is a second line; tokens
	  expire within the hour). Failing closed would log everyone out
	  whenever minioth restarts.
*/

// ErrRevoked is returned for a token minioth has revoked.
var ErrRevoked = errors.New("token revoked")

const revocationTTL = 30 * time.Second

type revocations struct {
	url    string // minioth's introspection endpoint
	client *http.Client

	mu       sync.Mutex
	seen     map[[32]byte]revEntry
	lastWarn time.Time
}

type revEntry struct {
	revoked bool
	until   time.Time // re-ask after this
}

func newRevocations(url string) *revocations {
	return &revocations{url: url, client: &http.Client{Timeout: 3 * time.Second}, seen: map[[32]byte]revEntry{}}
}

// revoked reports whether minioth has revoked raw (which expires at exp).
func (r *revocations) revoked(raw string, exp time.Time) bool {
	key := sha256.Sum256([]byte(raw))
	now := time.Now()
	r.mu.Lock()
	if e, ok := r.seen[key]; ok && now.Before(e.until) {
		r.mu.Unlock()

		return e.revoked
	}
	if len(r.seen) > 10000 { // forget expired entries now and then
		for k, e := range r.seen {
			if now.After(e.until) {
				delete(r.seen, k)
			}
		}
	}
	r.mu.Unlock()

	revoked, known := r.ask(raw)
	if !known {
		return false // minioth unreachable: accept on the signature (see above)
	}
	until := now.Add(revocationTTL)
	if revoked {
		until = exp
	}
	r.mu.Lock()
	r.seen[key] = revEntry{revoked: revoked, until: until}
	r.mu.Unlock()

	return revoked
}

// forget drops the cached answer for raw, so the next check asks minioth.
func (r *revocations) forget(raw string) {
	key := sha256.Sum256([]byte(raw))
	r.mu.Lock()
	delete(r.seen, key)
	r.mu.Unlock()
}

// ask queries minioth: (revoked, known). 2xx: valid; 400/401: rejected
// (revoked - the signature was already checked); anything else: unknown.
func (r *revocations) ask(raw string) (bool, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return false, false
	}
	req.Header.Set("Authorization", "Bearer "+raw)
	resp, err := r.client.Do(req)
	if err != nil {
		r.warn(err.Error())

		return false, false
	}
	_ = resp.Body.Close()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return false, true
	case resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized:
		return true, true
	default:
		r.warn(resp.Status)

		return false, false
	}
}

func (r *revocations) warn(what string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if time.Since(r.lastWarn) > time.Minute {
		r.lastWarn = time.Now()
		log.Printf("[authn] can't check token revocation (%s): accepting tokens on their signature", what)
	}
}
