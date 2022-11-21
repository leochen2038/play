package binders

import (
	"encoding/base64"
	"errors"
	"reflect"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

type jsonBinder struct {
	root gjson.Result
}

func GetBinderOfJson(data []byte) Binder {
	return &jsonBinder{root: gjson.GetBytes(data, "@this")}
}

func (b *jsonBinder) Name() string {
	return "json"
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
	return b.bindValue(v, s, b.root, "")
}

func (b *jsonBinder) bindValue(v reflect.Value, s reflect.StructField, source gjson.Result, preKey string) (err error) {
	var keys, required, fullKey string
	var item gjson.Result

	required, keys = s.Tag.Get("required"), s.Tag.Get("key")
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
				return errors.New("input: " + fullKey + " <" + s.Tag.Get("note") + "> " + err.Error())
			}
		} else if required == "true" {
			return errors.New("input: " + fullKey + " <" + s.Tag.Get("note") + "> is required")
		}
		return nil
	}

	switch s.Type.Kind() {
	case reflect.Struct:
		if s.Type.String() == "time.Time" {
			return setValWithGjson(v, s, item)
		} else {
			return b.bindStruct(v, item, fullKey)
		}
	case reflect.Slice:
		if err = b.bindSlice(v, item, fullKey); err != nil {
			return errors.New("input: " + fullKey + " " + err.Error() + " for json")
		}
		return nil
	default:
		return setValWithGjson(v, s, item)
	}
}

func (b *jsonBinder) bindStruct(v reflect.Value, source gjson.Result, preKey string) (err error) {
	count := v.Type().NumField()
	for i := 0; i < count; i++ {
		if err = b.bindValue(v.Field(i), v.Type().Field(i), source, preKey); err != nil {
			return
		}
	}

	return
}

func (b *jsonBinder) bindSlice(vField reflect.Value, source gjson.Result, preKey string) (err error) {
	fieldKind := vField.Type().Elem().Kind()
	if fieldKind == reflect.Struct {
		source.ForEach(func(key, value gjson.Result) bool {
			v := reflect.Indirect(reflect.New(vField.Type().Elem()))
			if err = b.bindStruct(v, value, preKey); err != nil {
				return false
			}
			vField.Set(reflect.Append(vField, v))
			return true
		})
	} else if fieldKind == reflect.Slice {
		source.ForEach(func(key, value gjson.Result) bool {
			v := reflect.Indirect(reflect.New(vField.Type().Elem()))
			if err = b.bindSlice(v, value, preKey); err != nil {
				return false
			}
			vField.Set(reflect.Append(vField, v))
			return true
		})
	} else {
		var elems = vField
		source.ForEach(func(key, value gjson.Result) bool {
			if elems, err = b.setSliceValueWithGJson(vField.Type().String(), elems, &value); err != nil {
				return false
			}
			vField.Set(elems)
			return true
		})
	}
	return
}

func setValWithGjson(vField reflect.Value, tField reflect.StructField, gValue gjson.Result) error {
	var val interface{}
	var err error

	if err = checkRegex(tField, gValue.String()); err != nil {
		return err
	}

	if tStr := strings.Trim(tField.Type.String(), "[]"); tStr == "interface {}" {
		val = gValue.Value()
	} else {
		if val, err = parseInterface(tStr, gValue.String(), tField); err != nil {
			return err
		}
	}
	vField.Set(reflect.ValueOf(val))
	return nil
}

func (b *jsonBinder) setSliceValueWithGJson(fieldType string, elems reflect.Value, value *gjson.Result) (reflect.Value, error) {
	if fieldType == "[]interface {}" {
		elems = reflect.Append(elems, reflect.ValueOf(value.Value()))
		return elems, nil
	}
	if fieldType == "[]uint8" && value.Type.String() == "String" {
		by, err := base64.StdEncoding.DecodeString(value.String())
		if err != nil {
			return elems, errors.New("is not base64 string")
		}
		elems.SetBytes(by)
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
