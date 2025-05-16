package test

import (
	"fmt"
	"reflect"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/zeebo/assert"
)

func TestIsEmpty(t *testing.T) {

	assert.True(t, ut.IsEmpty(reflect.ValueOf("")))
	assert.True(t, ut.IsEmpty(reflect.ValueOf(int(0))))
	assert.True(t, ut.IsEmpty(reflect.ValueOf(int8(0))))
	assert.True(t, ut.IsEmpty(reflect.ValueOf(int16(0))))
	assert.True(t, ut.IsEmpty(reflect.ValueOf(int32(0))))
	assert.True(t, ut.IsEmpty(reflect.ValueOf(int64(0))))
	assert.True(t, ut.IsEmpty(reflect.ValueOf(float32(0))))
	assert.True(t, ut.IsEmpty(reflect.ValueOf(float64(0))))
	assert.True(t, ut.IsEmpty(reflect.ValueOf(complex64(0))))
	assert.True(t, ut.IsEmpty(reflect.ValueOf(complex128(0))))
	assert.True(t, ut.IsEmpty(reflect.ValueOf(bool(false))))
	assert.True(t, ut.IsEmpty(reflect.ValueOf(struct{}{})))
	assert.True(t, ut.IsEmpty(reflect.ValueOf(map[any]any{})))

	assert.False(t, ut.IsEmpty(reflect.ValueOf("1")))
	assert.False(t, ut.IsEmpty(reflect.ValueOf(int(1))))
	assert.False(t, ut.IsEmpty(reflect.ValueOf(int8(1))))
	assert.False(t, ut.IsEmpty(reflect.ValueOf(int16(1))))
	assert.False(t, ut.IsEmpty(reflect.ValueOf(int32(1))))
	assert.False(t, ut.IsEmpty(reflect.ValueOf(int64(1))))
	assert.False(t, ut.IsEmpty(reflect.ValueOf(float32(1))))
	assert.False(t, ut.IsEmpty(reflect.ValueOf(float64(1))))
	assert.False(t, ut.IsEmpty(reflect.ValueOf(complex64(1))))
	assert.False(t, ut.IsEmpty(reflect.ValueOf(complex128(1))))
	assert.False(t, ut.IsEmpty(reflect.ValueOf(bool(true))))
	assert.False(t, ut.IsEmpty(reflect.ValueOf(struct{ t string }{t: "t"})))
	assert.False(t, ut.IsEmpty(reflect.ValueOf(map[any]any{"t": "t"})))

}

func TestMakeMap(t *testing.T) {
	var (
		names  []string
		values []any
		result map[string]any
	)

	names = []string{"id", "name", "address", "no"}
	values = []any{1, "", "johanes", 155}
	result = ut.MakeMapFrom(names, values)

	fmt.Println(result)
	assert.DeepEqual(t, result, map[string]any{"id": 1, "address": "johanes", "no": 155})

	names = []string{"id", "name", "address", "no"}
	values = []any{0, "zz", "", 2}
	result = ut.MakeMapFrom(names, values)

	fmt.Println(result)
	assert.DeepEqual(t, result, map[string]any{"name": "zz", "no": 2})

	names = []string{"id", "name", "address", "no"}
	values = []any{0, "", "", 0}
	result = ut.MakeMapFrom(names, values)

	fmt.Println(result)
	assert.DeepEqual(t, result, map[string]any{})

	names = []string{"id", "name", "address", "no"}
	values = []any{1, "1", "2", 3}
	result = ut.MakeMapFrom(names, values)

	fmt.Println(result)
	assert.DeepEqual(t, result, map[string]any{"id": 1, "name": "1", "address": "2", "no": 3})

}
