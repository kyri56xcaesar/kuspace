package utils

import (
	"log"
	"reflect"
)

// AssertStructNotEmptyUpon checks whether the specified fields of a struct are not empty (zero value).
//
// Arguments:
//   - strct: The struct (or pointer to struct) to be checked.
//   - assertm: A map where keys are field names (string) and values are booleans.
//     If the value is true, the corresponding field will be checked for non-emptiness.
//
// Returns:
//   - bool: Returns true if all specified fields are not empty; returns false if any specified field is empty or if the input is not a struct.
//
// utility package
//
// custom assert functions
func AssertStructNotEmptyUpon(strct any, assertm map[any]bool) bool {

	v := reflect.ValueOf(strct)
	t := reflect.TypeOf(strct)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}

	if v.Kind() != reflect.Struct {
		log.Printf("argument not a struct")
		return false
	}

	numfield := v.NumField()

	for i := range numfield {
		fieldType := t.Field(i)
		fieldValue := v.Field(i)

		if assertm[fieldType.Name] {
			// log.Printf("field: %v, value: %v", fieldType.Name, fieldValue)
			// check if this field is not empty
			if fieldValue.IsZero() {
				return false
			}
		}
	}

	return true
}
