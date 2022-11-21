package binders

import (
	"errors"
	"io"
	"mime/multipart"
	"net/url"
	"reflect"
	"strings"
)

type urlValueBinder struct {
	values url.Values
	keys   []string
	files  map[string][]*multipart.FileHeader
}

func GetBinderOfUrlValue(values url.Values, files map[string][]*multipart.FileHeader) Binder {
	binder := &urlValueBinder{values: values, files: files}
	for k, v := range values {
		if len(v) > 0 {
			binder.keys = append(binder.keys, k)
		}
	}
	for k, v := range files {
		if len(v) > 0 {
			binder.keys = append(binder.keys, k)
		}
	}
	return binder
}

func (b urlValueBinder) Name() string {
	return "urlvalue"
}

func (b urlValueBinder) Get(key string) interface{} {
	return b.values.Get(key)
}

func (b urlValueBinder) Bind(v reflect.Value, s reflect.StructField) error {
	return b.bindValue(v, s, "")
}

func (b urlValueBinder) bindValue(v reflect.Value, s reflect.StructField, preKey string) (err error) {
	var keys, required, skey, ckey string
	required, keys = s.Tag.Get("required"), s.Tag.Get("key")
	if keys == "" {
		keys = s.Name
	}

	if !v.CanInterface() {
		return
	}

	for _, v := range strings.Split(keys, ",") {
		ckey = strings.TrimSpace(v)
		if preKey != "" {
			ckey = preKey + "[" + ckey + "]"
			for _, ikey := range b.keys {
				if strings.HasPrefix(ikey, ckey) {
					skey = ikey
					break
				}
			}
		} else {
			for _, ikey := range b.keys {
				if ikey == ckey {
					skey = ikey
					break
				}
			}
		}
	}

	if skey == "" {
		if defaultValue := s.Tag.Get("default"); defaultValue != "" {
			if err = setValWithString(v, s, defaultValue); err != nil {
				return errors.New("input: " + ckey + " <" + s.Tag.Get("note") + "> " + err.Error())
			}
		} else if required == "true" {
			return errors.New("input: " + ckey + " <" + s.Tag.Get("note") + "> is required")
		}
		return nil
	}

	switch s.Type.Kind() {
	case reflect.Struct:
		if s.Type.String() == "time.Time" {
			return setValWithString(v, s, b.values.Get(skey))
		} else {
			return b.bindStructWithUrlValue(v, ckey)
		}
	case reflect.Slice:
		vType := v.Type().String()
		if (vType == "[]uint8" || vType == "[]byte" || vType == "[]int8") && b.files != nil {
			if fhs := b.files[skey]; len(fhs) > 0 {
				var f multipart.File
				if f, err = fhs[0].Open(); err != nil {
					return err
				}
				defer f.Close()
				buffer := make([]byte, fhs[0].Size)
				if _, err := io.ReadFull(f, buffer); err != nil {
					return err
				}
				v.Set(reflect.ValueOf(buffer))
				return nil
			} else {
				return errors.New("input: " + ckey + " <" + s.Tag.Get("note") + "> is required []byte or []int8")
			}
		} else {
			return b.bindSlice(v, s, ckey)
		}
	default:
		return setValWithString(v, s, b.values.Get(skey))
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

func (b urlValueBinder) bindSlice(vField reflect.Value, s reflect.StructField, preKey string) (err error) {
	if s.Type.Elem().Kind() == reflect.Struct {
		var keyList = map[string]struct{}{}
		for k := range b.values {
			if strings.HasPrefix(k, preKey) {
				if preKey, err := parseSliceKey(k, preKey); err != nil {
					return err
				} else {
					keyList[preKey] = struct{}{}
				}
			}
		}
		for k := range keyList {
			v := reflect.Indirect(reflect.New(vField.Type().Elem()))
			if err = b.bindStructWithUrlValue(v, k); err != nil {
				return err
			}
			vField.Set(reflect.Append(vField, v))
		}
	} else {
		for _, val := range b.values[preKey] {
			if elem, err := appendElem(vField, s, val, nil); err != nil {
				return errors.New("input: " + preKey + " <" + s.Tag.Get("note") + "> " + err.Error())
			} else {
				vField.Set(elem)
			}
		}
	}
	return
}
