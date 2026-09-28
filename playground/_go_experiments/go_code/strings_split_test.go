package gocode_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/zeebo/assert"
)

func TestSplit(t *testing.T) {
	subject := "/test/te"

	spl := strings.Split(strings.TrimPrefix(subject, "/"), "/")

	fmt.Println(spl)

	assert.Equal(t, len(spl), 2)

	fmt.Printf("spl: %v\n", spl[len(spl)-1])

	assert.Equal(t, 3, 2)
}
