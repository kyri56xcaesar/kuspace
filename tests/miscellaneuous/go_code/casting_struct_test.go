package gocode_test

import (
	"fmt"
	"testing"

	"github.com/zeebo/assert"
)

type TestStruct struct {
	Field1 int
	Field2 int
	Field3 string
}

func castToStruct(a any) TestStruct {
	fmt.Printf("a: %+v", a)
	z, ok := a.(TestStruct)
	fmt.Printf("z: %+v", z)
	if !ok {
		panic("failed to cast")
	}

	return z
}

func TestCast(t *testing.T) {
	ts := TestStruct{}

	assert.DeepEqual(t, ts, castToStruct(ts))
}
