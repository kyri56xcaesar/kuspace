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

	rStructType := reflect.TypeOf(r)
	rStructValue := reflect.ValueOf(r)

	numfield := rStructValue.NumField()
	// log.Printf("num_field: %v\n", numfield)

	for i := range numfield {

		fieldType := rStructType.Field(i)
		fieldValue := rStructValue.Field(i)

		// log.Printf("type: %v, field: %v", fieldType, fieldValue)

		if assertm[fieldType.Name] {
			log.Printf("field: %v, value: %v\n", fieldType.Name, fieldValue)
			// log.Print("isNil?: ", fieldValue.IsNil())
			log.Print("isZero?: ", fieldValue.IsZero())
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
