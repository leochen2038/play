package binders

import (
	"errors"
	"reflect"
)

type bytesBinder struct {
	data []byte
}

func GetBinderOfBytes(data []byte) Binder {
	return &bytesBinder{data: data}
}

func (b *bytesBinder) Name() string {
	return "bytes"
}
func (b *bytesBinder) Get(key string) interface{} {
	return b.data
}

func (b *bytesBinder) Bind(v reflect.Value, s reflect.StructField) error {
	bind := s.Tag.Get("required")
	if (v.Type().String() == "[]int8" || v.Type().String() == "[]byte") && len(b.data) > 0 {
		v.Set(reflect.ValueOf(b.data))
	} else {
		if bind == "true" {
			var key string
			if key = s.Tag.Get("key"); key == "" {
				key = s.Name
			}
			return errors.New("input: " + key + " <" + s.Tag.Get("note") + "> field is mismatch")
		}
	}
	return nil
}
