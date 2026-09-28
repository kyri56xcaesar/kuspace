package utils

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

var fuzzSecret = []byte("service-secret")

// FuzzVerifyWSTicket: a ticket opens only the job and role it was signed
// for, and nothing but a signed ticket opens anything.
func FuzzVerifyWSTicket(f *testing.F) {
	now := time.Unix(1_800_000_000, 0)
	good := SignWSTicket(fuzzSecret, "7", "consumer", "1001", now)
	f.Add(good, "7", "consumer")
	f.Add(good, "8", "consumer")
	f.Add(good, "7", "jack")
	f.Add(good+"x", "7", "consumer")
	f.Add("", "", "")
	f.Add("a.b", "7", "consumer")
	f.Fuzz(func(t *testing.T, ticket, jid, role string) {
		tk, err := VerifyWSTicket(fuzzSecret, ticket, jid, role, now)
		if err != nil {
			return
		}
		if ticket != good && ticket != SignWSTicket(fuzzSecret, jid, role, tk.UID, now) {
			t.Fatalf("accepted a ticket that wasn't signed: %q", ticket)
		}
		if ticket == good && (jid != "7" || !strings.EqualFold(role, "consumer")) {
			t.Fatalf("ticket for job 7/consumer opened %q/%q", jid, role)
		}
	})
}

// FuzzConfigValues: the loader parses or rejects any value, never panics,
// and lists never keep empty or padded items.
func FuzzConfigValues(f *testing.F) {
	for _, s := range []string{"", "1", "-1", "true", "maybe", "1.5", "a, b,,c", "  x  ", "9999999999999999999999", "\x00"} {
		f.Add(s)
	}
	kinds := []reflect.Value{
		reflect.New(reflect.TypeOf("")).Elem(), reflect.New(reflect.TypeOf(true)).Elem(),
		reflect.New(reflect.TypeOf(int(0))).Elem(), reflect.New(reflect.TypeOf(int32(0))).Elem(),
		reflect.New(reflect.TypeOf(int64(0))).Elem(), reflect.New(reflect.TypeOf(0.0)).Elem(),
		reflect.New(reflect.TypeOf([]byte{})).Elem(), reflect.New(reflect.TypeOf([]string{})).Elem(),
	}
	f.Fuzz(func(t *testing.T, raw string) {
		for _, v := range kinds {
			_ = setField(v, raw)
		}
		for _, item := range splitList(raw) {
			if item == "" || item != strings.TrimSpace(item) || strings.Contains(item, ",") {
				t.Fatalf("splitList(%q) kept %q", raw, item)
			}
		}
	})
}

// FuzzIsValidPath: an accepted path never has a ".." segment.
func FuzzIsValidPath(f *testing.F) {
	for _, s := range []string{"a/b.c", "../x", "a/../b", "..hidden/x", "a/..", "/", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if !IsValidPath(s) {
			return
		}
		for _, seg := range strings.Split(s, "/") {
			if seg == ".." {
				t.Fatalf("accepted %q", s)
			}
		}
	})
}
