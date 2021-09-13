package binder

import (
	"errors"
	"github.com/tidwall/gjson"
	"reflect"
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
		if preKey != "" {
			fullKey = preKey + "." + customKey
		} else {
			fullKey = customKey
		}
		if ex, ok := j.exData[customKey]; ok {
			if tField.Type.String() != reflect.TypeOf(ex).String() {
				return errors.New("input custom " + customKey + " type need " + tField.Type.String() + " but " + reflect.TypeOf(ex).String() + " given")
			}
			vField.Set(reflect.ValueOf(ex))
			continue
		}

		item = source.Get(customKey)
		if !item.Exists() || item.Type == gjson.Null {
			if defaultValue := tField.Tag.Get("default"); defaultValue != "" {
				if err = setVal(vField, tField, defaultValue); err != nil {
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
			if item.ForEach(func(key, value gjson.Result) bool {
				count++
				var elem reflect.Value
				if elem, err = appendElem(vField, tField, value.String()); err != nil {
					return false
				}
				vField.Set(elem)

				return true
			}); err != nil {
				return errors.New("input <" + fullKey + "> " + err.Error())
			}
			if count == 0 && bind == "required" {
				return errors.New("input <" + fullKey + "> field is mismatch")
			}
		} else {
			if err = setVal(vField, tField, item.String()); err != nil {
				return errors.New("input <" + fullKey + "> " + err.Error())
			}
		}
	}
	return
}