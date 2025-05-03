package test

import (
	"strings"
	"testing"

	"github.com/zeebo/assert"
)

func TestR(t *testing.T) {
	logic := "test/app:v1"
	lang := logic[:strings.Index(logic+":", ":")]

	assert.Equal(t, lang, "test/app")

	logic = "test"
	lang = logic[:strings.Index(logic+":", ":")]

	assert.Equal(t, lang, "test")

	logic = "test/app"
	lang = logic[:strings.Index(logic+":", ":")]

	assert.Equal(t, lang, "test/app")

	logic = "test::"
	lang = logic[:strings.Index(logic+":", ":")]

	assert.Equal(t, lang, "test")

	logic = ":"
	lang = logic[:strings.Index(logic+":", ":")]

	assert.Equal(t, lang, "")
}
