package utils

import (
	"reflect"
	"testing"
)

func TestIsValidPath(t *testing.T) {
	cases := map[string]bool{
		"uspace-default/data.csv": true,
		"a/b.c/d_e-f":             true,
		"..hidden/x":              true, // ".." inside a name is fine, only a whole segment is not
		"a/../b":                  false,
		"../etc/passwd":           false,
		"a/..":                    false,
		"a b":                     false,
		"a;rm -rf":                false,
		"":                        false,
	}
	for in, want := range cases {
		if got := IsValidPath(in); got != want {
			t.Errorf("IsValidPath(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestMakeMapFromSkipsEmptyValues(t *testing.T) {
	got := MakeMapFrom([]string{"id", "name", "address", "no"}, []any{1, "", "johanes", 0})
	want := map[string]any{"id": 1, "address": "johanes"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MakeMapFrom = %v, want %v", got, want)
	}
	if MakeMapFrom([]string{"a"}, nil) != nil {
		t.Error("mismatched lengths accepted")
	}
}
