package binder

import (
	"errors"
	"github.com/tidwall/gjson"
	"reflect"
	"strconv"
	"strings"
)

type JsonBinder struct {
	json   gjson.Result
	exData map[string]interface{}
}

func NewJsonBinder(data []byte) *JsonBinder {
	parser := &JsonBinder{json: gjson.GetBytes(data, "@this"), exData: make(map[string]interface{})}
	return parser
}

func (j *JsonBinder) Bind(v reflect.Value) error {
	if v.CanSet() {
		return j.bindGJson(v, j.json, "", "")
	}
	return nil
}

func (j *JsonBinder) Get(key string) (val interface{}, err error) {
	if result := j.json.Get(key); result.Exists() {
		val = result.Value()
	} else if key == "" {
		val = j.json.Value()
	} else {
		err = errors.New("can not find key|" + key)
	}

	return
}

func (j *JsonBinder) Set(key string, val interface{}) {
	j.exData[key] = val
}

func (j *JsonBinder) bindGJson(v reflect.Value, source gjson.Result, required string, preKey string) (err error) {
	var tField reflect.StructField
	var vField reflect.Value
	var item gjson.Result
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
			if ex, ok := j.exData[v]; ok {
				if tField.Type.String() != reflect.TypeOf(ex).String() {
					return errors.New("input custom " + v + " type need " + tField.Type.String() + " but " + reflect.TypeOf(ex).String() + " given")
				}
				vField.Set(reflect.ValueOf(ex))
				//continue
				goto NEXT
			}

			item = source.Get(v)
			item.Exists()
			if item.Exists() {
				break
			}
		}
		if !item.Exists() || item.Type == gjson.Null {
			if defaultValue := tField.Tag.Get("default"); defaultValue != "" {
				if err = setVal(vField, tField, defaultValue, &item); err != nil {
					return errors.New("input <" + customKey + "> " + err.Error())
				}
			} else if bind == "required" {
				return errors.New("input <" + fullKey + "> field is mismatch")
			}
			continue
		}

		if tField.Type.Kind() == reflect.Struct && tField.Type.String() != "time.Time" {
			if err = j.bindGJson(vField, item, bind, fullKey); err != nil {
				return
			}
			continue
		}
		if tField.Type.Kind() == reflect.Slice && vField.Type().Elem().Kind() == reflect.Struct && vField.Type().Elem().String() != "time.Time" {
			var count int
			item.ForEach(func(key, value gjson.Result) bool {
				count++
				v := reflect.Indirect(reflect.New(vField.Type().Elem()))
				if err = j.bindGJson(v, value, bind, fullKey); err != nil {
					return false
				}
				vField.Set(reflect.Append(vField, v))
				return true
			})
			if err != nil {
				return err
			}
			if count == 0 && bind == "required" {
				return errors.New("input <" + fullKey + "> field is mismatch")
			}
			continue
		}

		if tField.Type.Kind() == reflect.Slice {
			var count int
			var elems = vField
			if item.ForEach(func(key, value gjson.Result) bool {
				count++
				//var elem reflect.Value
				//if elem, err = appendElem(vField, tField, value.String()); err != nil {
				//	return false
				//}
				if elems, err = setSliceValueWithGJson(vField.Type().String(), elems, &value); err != nil {
					return false
				}
				vField.Set(elems)

				return true
			}); err != nil {
				return errors.New("input <" + fullKey + "> " + err.Error())
			}
			if count == 0 && bind == "required" {
				return errors.New("input <" + fullKey + "> field is mismatch")
			}
		} else {
			if err = setVal(vField, tField, item.String(), &item); err != nil {
				return errors.New("input <" + fullKey + "> " + err.Error())
			}
		}
	NEXT:
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
