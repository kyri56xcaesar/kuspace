package gocode_test

import (
	"fmt"
	"testing"

	"github.com/zeebo/assert"
)

func testFunc(args ...any) {
	fmt.Printf("args: %+v\n", args)
	// m := args[3].(map[string]int)
	// fmt.Printf("map arg: %v\n", m["hello"])
}

func TestMArgs(t *testing.T) {
	testFunc("1,3", 5, struct{ t, t2 string }{"test", "test"}, map[string]int{"hello": 1})
	assert.Equal(t, "f", 5)
}
