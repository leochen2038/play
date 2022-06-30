package binders

import (
	"errors"
	"reflect"
)

type bytesBinder struct {
	data []byte
}

func NewBytesBinder(data []byte) Binder {
	return &bytesBinder{data: data}
}

func (b *bytesBinder) Get(key string) interface{} {
	return b.data
}

func (b *bytesBinder) Bind(v reflect.Value, s reflect.StructField) error {
	bind := s.Tag.Get("bind")
	if (v.Type().String() == "[]int8" || v.Type().String() == "[]byte") && len(b.data) > 0 {
		v.Set(reflect.ValueOf(b.data))
	} else {
		if bind == "required" {
			var key string
			if key = s.Tag.Get("key"); key == "" {
				key = s.Name
			}
			return errors.New("input <" + key + "> field is mismatch")
		}
	}
	return nil
}
