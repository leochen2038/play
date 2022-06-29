package binders

import (
	"errors"
	"net/url"
	"reflect"
	"strings"
)

type urlValueBinder struct {
	values url.Values
	keys   []string
}

func NewUrlValueBinder(values url.Values) Binder {
	binder := &urlValueBinder{values: values}
	for k, _ := range values {
		binder.keys = append(binder.keys, k)
	}
	return binder
}

func (b urlValueBinder) Get(key string) interface{} {
	return nil
}

func (b urlValueBinder) Bind(v reflect.Value, s reflect.StructField) error {
	return nil
}

func (b urlValueBinder) bindValue(v reflect.Value, s reflect.StructField, preKey string) (err error) {
	var keys, bind, fullKey string
	bind, keys = s.Tag.Get("bind"), s.Tag.Get("key")
	if keys == "" {
		keys = s.Name
	}

	if !v.CanInterface() {
		return
	}

	for _, v := range strings.Split(keys, ",") {
		foo := strings.TrimSpace(v)
		if preKey != "" {
			foo = preKey + "[" + foo + "]"
		}
		for _, ikey := range b.keys {
			if ikey == foo {
				fullKey = ikey
				break
			}
		}
	}
	if fullKey == "" {
		if defaultValue := s.Tag.Get("default"); defaultValue != "" {
			if err = setValWithString(v, s, defaultValue); err != nil {
				return errors.New("input field <" + fullKey + "> " + err.Error())
			}
		} else if bind == "required" {
			return errors.New("input field <" + fullKey + "> is required")
		}
		return nil
	}

	switch s.Type.Kind() {
	case reflect.Struct:
		if s.Type.String() == "time.Time" {
			return setValWithString(v, s, b.values.Get(fullKey))
		} else {
			return b.bindStructWithUrlValue(v, fullKey)
		}
	case reflect.Slice:
		return b.bindSlice(v, s, fullKey)
	default:
		return setValWithString(v, s, b.values.Get(fullKey))
	}
}

func (b urlValueBinder) bindStructWithUrlValue(v reflect.Value, preKey string) (err error) {
	count := v.Type().NumField()
	for i := 0; i < count; i++ {
		if err = b.bindValue(v.Field(i), v.Type().Field(i), preKey); err != nil {
			return err
		}
	}
	return
}

func (b urlValueBinder) bindSlice(v reflect.Value, s reflect.StructField, preKey string) (err error) {
	if s.Type.Kind() == reflect.Struct {
		var keyList = make(map[string]struct{}, 8)
		for k, _ := range b.values {
			if strings.HasPrefix(k, preKey) {
				if preKey, err := parseSliceKey(k, preKey); err != nil {
					return err
				} else {
					keyList[preKey] = struct{}{}
				}
			}
		}

		for k := range keyList {
			v := reflect.Indirect(reflect.New(v.Type().Elem()))
			if err = b.bindValue(v, s, k); err != nil {
				return err
			}
			v.Set(reflect.Append(v, v))
		}
	} else {
		for _, val := range b.values[preKey] {
			if elem, err := appendElem(v, s, val, nil); err != nil {
				return errors.New("input <" + preKey + "> " + err.Error())
			} else {
				v.Set(elem)
			}
		}
	}
	return
}
