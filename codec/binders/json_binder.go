package binders

import (
	"errors"
	"reflect"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

type jsonBinder struct {
	root gjson.Result
}

func NewJsonDecoder(data []byte) Binder {
	return &jsonBinder{root: gjson.GetBytes(data, "@this")}
}

func (b *jsonBinder) Get(key string) (val interface{}) {
	if key == "" {
		val = b.root.Value()
	} else if result := b.root.Get(key); result.Exists() {
		val = result.Value()
	}
	return
}

func (b *jsonBinder) Bind(v reflect.Value, s reflect.StructField) error {
	return bindValue(v, s, b.root, "")
}

func bindValue(v reflect.Value, s reflect.StructField, source gjson.Result, preKey string) (err error) {
	var keys, bind, fullKey string
	var item gjson.Result

	bind, keys = s.Tag.Get("bind"), s.Tag.Get("key")
	if keys == "" {
		keys = s.Name
	}
	if !v.CanInterface() {
		return
	}

	for _, v := range strings.Split(keys, ",") {
		key := strings.TrimSpace(v)
		if fullKey == "" {
			if preKey != "" {
				fullKey = preKey + "." + key
			} else {
				fullKey = key
			}
		}
		if item = source.Get(key); item.Exists() {
			break
		}
	}

	if !item.Exists() || item.Type == gjson.Null {
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
			return setValWithGjson(v, s, item)
		} else {
			return bindStruct(v, item, fullKey)
		}
	case reflect.Slice:
		return bindSlice(v, s, item, fullKey)
	default:
		return setValWithGjson(v, s, item)
	}
}

func bindStruct(v reflect.Value, source gjson.Result, preKey string) (err error) {
	count := v.Type().NumField()
	for i := 0; i < count; i++ {
		if err = bindValue(v.Field(i), v.Type().Field(i), source, preKey); err != nil {
			return
		}
	}

	return
}

func bindSlice(v reflect.Value, s reflect.StructField, source gjson.Result, preKey string) (err error) {
	if s.Type.Kind() == reflect.Struct {
		source.ForEach(func(key, value gjson.Result) bool {
			v := reflect.Indirect(reflect.New(v.Type().Elem()))
			if err = bindStruct(v, value, preKey); err != nil {
				return false
			}
			v.Set(reflect.Append(v, v))
			return true
		})
	} else {
		var elems = v
		source.ForEach(func(key, value gjson.Result) bool {
			if elems, err = setSliceValueWithGJson(v.Type().String(), elems, &value); err != nil {
				return false
			}
			v.Set(elems)
			return true
		})
	}
	return
}

func setSliceValueWithGJson(fieldType string, elems reflect.Value, value *gjson.Result) (reflect.Value, error) {
	if fieldType == "[]interface {}" {
		elems = reflect.Append(elems, reflect.ValueOf(value.Value()))
		return elems, nil
	}
	if fieldType != "[]string" && value.Type.String() != "Number" {
		if _, err := strconv.ParseFloat(value.Str, 64); err != nil {
			return elems, errors.New("data type need number")
		}
	}
	switch fieldType {
	case "[]string":
		elems = reflect.Append(elems, reflect.ValueOf(value.String()))
	case "[]int8":
		elems = reflect.Append(elems, reflect.ValueOf(int8(value.Int())))
	case "[]int32":
		elems = reflect.Append(elems, reflect.ValueOf(int32(value.Int())))
	case "[]int64":
		elems = reflect.Append(elems, reflect.ValueOf(value.Int()))
	case "[]int":
		elems = reflect.Append(elems, reflect.ValueOf(int(value.Int())))
	case "[]uint8":
		elems = reflect.Append(elems, reflect.ValueOf(uint8(value.Uint())))
	case "[]uint32":
		elems = reflect.Append(elems, reflect.ValueOf(uint32(value.Uint())))
	case "[]uint64":
		elems = reflect.Append(elems, reflect.ValueOf(value.Uint()))
	case "[]uint":
		elems = reflect.Append(elems, reflect.ValueOf(uint(value.Uint())))
	case "[]float32":
		elems = reflect.Append(elems, reflect.ValueOf(float32(value.Float())))
	case "[]float64":
		elems = reflect.Append(elems, reflect.ValueOf(value.Float()))
	}

	return elems, nil
}
