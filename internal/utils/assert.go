package utils

import (
	"log"
	"reflect"
)

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
		field_type := t.Field(i)
		field_value := v.Field(i)

		if assertm[field_type.Name] {
			// log.Printf("field: %v, value: %v", field_type.Name, field_value)
			// check if this field is not empty
			if field_value.IsZero() {
				return false
			}
		}
	}

	return true
}
