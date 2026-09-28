package frontendapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

/*
	Upstream services (uspace, minioth)

	Every call goes through one client, which sets the credentials the same
	way each time. Calls made for a logged-in user carry the user's own
	token: uspace and minioth verify it and take the identity from it, so
	frontapp never asserts who the user is. Only calls no user can make
	(e.g. setting up storage for someone who just registered) use the
	service secret.
*/

type upstream struct {
	base   string // e.g. http://uspace:8079
	secret []byte // service secret
	client *http.Client
}

func newUpstream(base string, secret []byte) *upstream {
	return &upstream{base: base, secret: secret, client: &http.Client{Timeout: 5 * time.Minute}}
}

// request describes one upstream call.
type request struct {
	method string
	path   string // e.g. /api/v1/resources
	query  url.Values
	body   io.Reader
	ctype  string            // Content-Type of body
	json   any               // marshalled as the body when set
	token  string            // the user's access token
	svc    bool              // authenticate as a service instead
	target string            // uspace Access-Target "vid:vname:target" (identity comes from the token)
	header map[string]string // extra headers
}

func (u *upstream) do(ctx context.Context, r request) (*http.Response, error) {
	if r.json != nil {
		b, err := json.Marshal(r.json)
		if err != nil {
			return nil, err
		}
		r.body, r.ctype = bytes.NewReader(b), "application/json"
	}
	target := u.base + r.path
	if len(r.query) > 0 {
		target += "?" + r.query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, r.method, target, r.body)
	if err != nil {
		return nil, err
	}
	if r.ctype != "" {
		req.Header.Set("Content-Type", r.ctype)
	}
	switch {
	case r.token != "":
		req.Header.Set("Authorization", "Bearer "+r.token)
	case r.svc:
		req.Header.Set("X-Service-Secret", string(u.secret))
	}
	if r.target != "" {
		req.Header.Set("Access-Target", r.target)
	}
	for k, v := range r.header {
		req.Header.Set(k, v)
	}

	return u.client.Do(req)
}

// decode runs the call and decodes a 2xx JSON answer into out. Other
// statuses become an *upstreamError carrying the upstream's message.
func (u *upstream) decode(ctx context.Context, r request, out any) error {
	resp, err := u.do(ctx, r)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return newUpstreamError(resp)
	}
	if out == nil {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

// upstreamError is a non-2xx answer from an upstream service.
type upstreamError struct {
	status int
	msg    string
}

func (e *upstreamError) Error() string { return fmt.Sprintf("upstream %d: %s", e.status, e.msg) }

func newUpstreamError(resp *http.Response) *upstreamError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var m struct {
		Error string `json:"error"`
	}
	msg := string(body)
	if json.Unmarshal(body, &m) == nil && m.Error != "" {
		msg = m.Error
	}

	return &upstreamError{status: resp.StatusCode, msg: msg}
}

// fail answers a failed upstream call: its own status and message for an
// upstream error, 502 when the service couldn't be reached.
func fail(c *gin.Context, what string, err error) {
	var ue *upstreamError
	if errors.As(err, &ue) {
		c.JSON(ue.status, gin.H{"error": ue.msg})

		return
	}
	log.Printf("[frontapp] %s: %v", what, err)
	c.JSON(http.StatusBadGateway, gin.H{"error": what + ": service unavailable"})
}

// relay streams an upstream answer to the client: status, the listed
// headers (plus Content-Type and Content-Length), and the body.
func relay(c *gin.Context, resp *http.Response, headers ...string) {
	defer func() { _ = resp.Body.Close() }()
	for _, h := range append([]string{"Content-Type", "Content-Length"}, headers...) {
		if v := resp.Header.Get(h); v != "" {
			c.Header(h, v)
		}
	}
	c.Status(resp.StatusCode)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		log.Printf("[frontapp] relay: %v", err)
	}
}

// userToken is the verified session token (set by requireRole).
func userToken(c *gin.Context) string {
	return c.GetString("accessToken")
}

// accessTarget renders uspace's Access-Target "what": vid:vname:target.
// The identity part is left to uspace, which takes it from the token.
func accessTarget(vname, target string) string {
	return "0:" + vname + ":" + target
}
