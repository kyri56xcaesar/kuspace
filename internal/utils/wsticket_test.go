package utils

import (
	"strings"
	"testing"
	"time"
)

func TestWSTicket(t *testing.T) {
	secret := []byte("service-secret")
	now := time.Unix(1_800_000_000, 0)
	tk := SignWSTicket(secret, "42", "Consumer", "1001", now)

	got, err := VerifyWSTicket(secret, tk, "42", "consumer", now.Add(30*time.Second))
	if err != nil || got.UID != "1001" || got.JID != "42" {
		t.Fatalf("valid ticket: %+v, %v", got, err)
	}

	p, s, _ := strings.Cut(tk, ".")
	other := SignWSTicket(secret, "43", "consumer", "1001", now)
	op, _, _ := strings.Cut(other, ".")
	for name, c := range map[string]struct {
		ticket, jid, role string
		at                time.Time
		secret            []byte
	}{
		"expired":         {tk, "42", "consumer", now.Add(2 * WSTicketTTL), secret},
		"another job":     {tk, "43", "consumer", now, secret},
		"another role":    {tk, "42", "producer", now, secret},
		"other secret":    {tk, "42", "consumer", now, []byte("nope")},
		"swapped payload": {op + "." + s, "43", "consumer", now, secret},
		"garbage":         {"x.y", "42", "consumer", now, secret},
		"no signature":    {p, "42", "consumer", now, secret},
	} {
		if _, err := VerifyWSTicket(c.secret, c.ticket, c.jid, c.role, c.at); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
