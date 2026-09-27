package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

/*
	Tickets for the job websocket (wss).

	Browsers connect to wss directly and wss can't see frontapp's session, so
	frontapp - after checking the user may watch the job - hands out a
	short-lived ticket: "<jid>|<role>|<uid>|<unix expiry>" plus an HMAC-SHA256
	over it. wss only checks the signature, the expiry and that the ticket is
	for the job and role being asked for: no database, no calls out.

	The key is derived from the service secret both services already hold,
	domain-separated so a ticket can't be mistaken for anything else.
*/

// WSTicketTTL is how long a ticket can be used to open a connection.
const WSTicketTTL = time.Minute

var (
	errTicketFormat  = errors.New("malformed ticket")
	errTicketSig     = errors.New("bad ticket signature")
	errTicketExpired = errors.New("ticket expired")
	errTicketScope   = errors.New("ticket is for another job or role")
)

// WSTicket is what a valid ticket grants.
type WSTicket struct {
	JID  string
	Role string
	UID  string
}

func wsTicketKey(serviceSecret []byte) []byte {
	m := hmac.New(sha256.New, serviceSecret)
	m.Write([]byte("kuspace wss ticket v1"))

	return m.Sum(nil)
}

func wsTicketMAC(key []byte, payload string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(payload))

	return m.Sum(nil)
}

// SignWSTicket issues a ticket letting uid join job jid's stream as role.
func SignWSTicket(serviceSecret []byte, jid, role, uid string, now time.Time) string {
	payload := strings.Join([]string{jid, strings.ToLower(role), uid, strconv.FormatInt(now.Add(WSTicketTTL).Unix(), 10)}, "|")
	enc := base64.RawURLEncoding

	return enc.EncodeToString([]byte(payload)) + "." + enc.EncodeToString(wsTicketMAC(wsTicketKey(serviceSecret), payload))
}

// VerifyWSTicket checks a ticket's signature and expiry and that it was
// issued for this job and role.
func VerifyWSTicket(serviceSecret []byte, ticket, jid, role string, now time.Time) (WSTicket, error) {
	enc := base64.RawURLEncoding
	p, s, ok := strings.Cut(ticket, ".")
	if !ok {
		return WSTicket{}, errTicketFormat
	}
	payloadB, err1 := enc.DecodeString(p)
	sig, err2 := enc.DecodeString(s)
	if err1 != nil || err2 != nil {
		return WSTicket{}, errTicketFormat
	}
	payload := string(payloadB)
	if !hmac.Equal(sig, wsTicketMAC(wsTicketKey(serviceSecret), payload)) {
		return WSTicket{}, errTicketSig
	}
	parts := strings.Split(payload, "|")
	if len(parts) != 4 {
		return WSTicket{}, errTicketFormat
	}
	exp, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return WSTicket{}, errTicketFormat
	}
	if now.Unix() > exp {
		return WSTicket{}, errTicketExpired
	}
	if parts[0] != jid || parts[1] != strings.ToLower(role) {
		return WSTicket{}, fmt.Errorf("%w (ticket: job %s as %s)", errTicketScope, parts[0], parts[1])
	}

	return WSTicket{JID: parts[0], Role: parts[1], UID: parts[2]}, nil
}
