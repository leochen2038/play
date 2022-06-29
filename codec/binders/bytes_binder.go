package binders

import (
	"errors"
	"reflect"
)

type bytesDecoder struct {
	data []byte
}

func NewBytesDecoder(data []byte) Binder {
	return &bytesDecoder{data: data}
}

func (decoder *bytesDecoder) Get(key string) interface{} {
	return decoder.data
}

func (decoder *bytesDecoder) Bind(v reflect.Value, s reflect.StructField) error {
	bind := s.Tag.Get("bind")
	if (v.Type().String() == "[]int8" || v.Type().String() == "[]byte") && len(decoder.data) > 0 {
		v.Set(reflect.ValueOf(decoder.data))
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
