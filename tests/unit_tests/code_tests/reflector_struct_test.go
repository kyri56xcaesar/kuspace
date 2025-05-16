package test

import (
	"bytes"
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/zeebo/assert"
)

func captureOutput(f func()) string {
	var buf bytes.Buffer
	log.SetFlags(0)
	log.SetOutput(&buf)
	f()
	log.SetOutput(os.Stderr)
	return buf.String()
}

type testStruct struct {
	field1 int
	field2 int
	field3 float64
	field4 float32
	field5 string
}

func (r testStruct) AssertUpon(assertm map[any]bool) bool {

	// log.Printf("current assertmap: %v\n", assertm)

	r_struct_type := reflect.TypeOf(r)
	r_struct_value := reflect.ValueOf(r)

	numfield := r_struct_value.NumField()
	// log.Printf("num_field: %v\n", numfield)

	for i := range numfield {

		field_type := r_struct_type.Field(i)
		field_value := r_struct_value.Field(i)

		// log.Printf("type: %v, field: %v", field_type, field_value)

		if assertm[field_type.Name] {
			log.Printf("field: %v, value: %v\n", field_type.Name, field_value)
			// log.Print("isNil?: ", field_value.IsNil())
			log.Print("isZero?: ", field_value.IsZero())
		}
	}

	return true
}
func TestReflect(t *testing.T) {
	output := captureOutput(func() {
		test := map[any]bool{
			"field1": false,
			"field2": false,
			"field3": false,
			"field4": false,
			"field5": true,
		}

		ts := testStruct{
			field1: 0,
			field2: 2,
			field3: 3.0,
			field4: 4.0,
			field5: "salkjhglsajh",
		}

		ts.AssertUpon(test)
	})

	log.Print("output: ", output)

	assert.Equal(t, output, "hello")
}
