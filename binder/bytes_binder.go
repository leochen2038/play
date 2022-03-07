package binder

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type BytesBinder struct {
	data   []byte
	exData map[string]interface{}
}

func NewBytesBinder(data []byte) *BytesBinder {
	return &BytesBinder{data: data, exData: make(map[string]interface{})}
}

func (j *BytesBinder) Bind(v reflect.Value) error {
	if v.CanSet() {
		return j._bind(v, "", "")
	}
	return nil
}

func (j *BytesBinder) Get(key string) (val interface{}, err error) {
	if key == "" {
		return j.data, nil
	} else {
		var ok bool
		if val, ok = j.exData[key]; ok {
			return val, nil
		}
	}
	return nil, errors.New("can not find key|" + key)
}

func (j *BytesBinder) Set(key string, val interface{}) {
	j.exData[key] = val
	return
}

func (j *BytesBinder) _bind(v reflect.Value, required string, preKey string) (err error) {
	var tField reflect.StructField
	var vField reflect.Value
	var fieldCount = v.Type().NumField()
	var customKey string
	var bind string // required, optional
	var fullKey string
	for i := 0; i < fieldCount; i++ {
		if vField, tField = v.Field(i), v.Type().Field(i); !vField.CanInterface() {
			continue
		}

		bind = required
		if tField.Tag.Get("bind") != "" {
			bind = tField.Tag.Get("bind")
		}

		if customKey = tField.Tag.Get("key"); customKey == "" {
			customKey = tField.Name
		}
		customKeys := strings.Split(customKey, ",")
		for _, v := range customKeys {
			if preKey != "" {
				fullKey = preKey + "." + v
			} else {
				fullKey = v
			}
			if ex, ok := j.exData[fullKey]; ok {
				if tField.Type.String() != reflect.TypeOf(ex).String() {
					return errors.New("input custom " + v + " type need " + tField.Type.String() + " but " + reflect.TypeOf(ex).String() + " given")
				}
				vField.Set(reflect.ValueOf(ex))
				//continue
				goto NEXT
			}
		}
		fmt.Println(tField.Type.String())
		fmt.Println(vField.Type().String())
		if vField.Type().String() == "[]uint8" && len(j.data) > 0 {
			vField.Set(reflect.ValueOf(j.data))
		} else {
			if bind == "required" {
				return errors.New("input <" + fullKey + "> field is mismatch")
			}
		}

	NEXT:
	}
	return
}
