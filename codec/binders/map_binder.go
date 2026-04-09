package binders

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type mapBinder struct {
	data map[string]any
}

func GetBinderOfMap(data map[string]any) Binder {
	return &mapBinder{data: data}
}

func (b *mapBinder) Name() string {
	return "map"
}

func (b *mapBinder) Get(key string) interface{} {
	if key == "" {
		return b.data
	}
	return b.data[key]
}

func (b *mapBinder) Bind(v reflect.Value, s reflect.StructField) error {
	return b.bindValue(v, s, b.data, "")
}

func (b *mapBinder) bindValue(v reflect.Value, s reflect.StructField, source map[string]any, preKey string) error {
	var keys, required, fullKey string
	var val any
	var found bool

	required, keys = s.Tag.Get("required"), s.Tag.Get("key")
	if keys == "" {
		keys = s.Name
	}
	if !v.CanInterface() {
		return nil
	}

	for _, k := range strings.Split(keys, ",") {
		key := strings.TrimSpace(k)
		if fullKey == "" {
			if preKey != "" {
				fullKey = preKey + "." + key
			} else {
				fullKey = key
			}
		}
		if val, found = source[key]; found {
			break
		}
	}

	if !found || val == nil {
		if defaultValue := s.Tag.Get("default"); defaultValue != "" {
			if err := setValWithString(v, s, defaultValue); err != nil {
				return errors.New("input: " + fullKey + " <" + s.Tag.Get("note") + "> " + err.Error())
			}
			return nil
		} else if required == "true" {
			return errors.New("input: " + fullKey + " <" + s.Tag.Get("note") + "> is required")
		}
		return nil
	}

	switch s.Type.Kind() {
	case reflect.Struct:
		if s.Type.String() == "time.Time" {
			return setValWithString(v, s, fmt.Sprint(val))
		}
		if sub, ok := val.(map[string]any); ok {
			return b.bindStruct(v, sub, fullKey)
		}
		return setValWithString(v, s, fmt.Sprint(val))
	case reflect.Slice:
		if arr, ok := val.([]any); ok {
			return b.bindSlice(v, s, arr, fullKey)
		}
		return nil
	default:
		return setValWithString(v, s, fmt.Sprint(val))
	}
}

func (b *mapBinder) bindStruct(v reflect.Value, source map[string]any, preKey string) error {
	for i := 0; i < v.Type().NumField(); i++ {
		if err := b.bindValue(v.Field(i), v.Type().Field(i), source, preKey); err != nil {
			return err
		}
	}
	return nil
}

func (b *mapBinder) bindSlice(vField reflect.Value, s reflect.StructField, arr []any, preKey string) error {
	elemKind := vField.Type().Elem().Kind()
	for _, item := range arr {
		if elemKind == reflect.Struct {
			if sub, ok := item.(map[string]any); ok {
				elem := reflect.Indirect(reflect.New(vField.Type().Elem()))
				if err := b.bindStruct(elem, sub, preKey); err != nil {
					return err
				}
				vField.Set(reflect.Append(vField, elem))
			}
		} else {
			if elem, err := appendElem(vField, s, fmt.Sprint(item), nil); err != nil {
				return errors.New("input: " + preKey + " <" + s.Tag.Get("note") + "> " + err.Error())
			} else {
				vField.Set(elem)
			}
		}
	}
	return nil
}
