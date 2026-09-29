package config

import (
	"reflect"
	"strings"
	"testing"
)

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
