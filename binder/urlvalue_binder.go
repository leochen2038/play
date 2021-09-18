package binder

import (
	"errors"
	"net/url"
	"reflect"
	"strings"
)

type UrlValueBinder struct {
	values url.Values
	exData map[string]interface{}
}

func NewUrlValueBinder(values url.Values) *UrlValueBinder {
	parser := &UrlValueBinder{values: values, exData: make(map[string]interface{})}
	return parser
}

func (parser *UrlValueBinder) Set(key string, val interface{}) {
	parser.exData[key] = val
}

func (parser *UrlValueBinder) Get(key string) (interface{}, error) {
	return parser.values.Get(key), nil
}

func (parser *UrlValueBinder) Bind(v reflect.Value) error {
	if v.CanSet() {
		return parser.bindValues(v, "", "")
	}
	return nil
}

func (parser *UrlValueBinder) bindValues(v reflect.Value, prefix string, required string) (err error) {
	var tField reflect.StructField
	var vField reflect.Value
	var item []string
	var fieldCount = v.Type().NumField()
	var customKeys []string
	var customKeystr string
	var bind string // required, optional

	for i := 0; i < fieldCount; i++ {
		if vField, tField = v.Field(i), v.Type().Field(i); !vField.CanInterface() {
			continue
		}

		bind = required
		if tField.Tag.Get("bind") != "" {
			bind = tField.Tag.Get("bind")
		}
		if customKeystr = tField.Tag.Get("key"); customKeystr == "" {
			customKeystr = tField.Name
		}

		customKeys = strings.Split(customKeystr, ",")
		for _, customKey := range customKeys {
			if prefix != "" {
				customKey = prefix + "[" + customKey + "]"
			}

			if ex, ok := parser.exData[customKey]; ok {
				if tField.Type.String() != reflect.TypeOf(ex).String() {
					return errors.New("input custom " + customKey + " type need " + tField.Type.String() + " but " + reflect.TypeOf(ex).String() + " given")
				}
				vField.Set(reflect.ValueOf(ex))
				goto NEXT
			}

			if tField.Type.Kind() == reflect.Struct && tField.Type.String() != "time.Time" {
				if err = parser.bindValues(vField, customKey, bind); err != nil {
					return
				}
				goto NEXT
			}
			if tField.Type.Kind() == reflect.Slice && vField.Type().Elem().Kind() == reflect.Struct && vField.Type().Elem().String() != "time.Time" {
				var keyList = make(map[string]struct{}, 8)
				for k, _ := range parser.values {
					if strings.HasPrefix(k, customKey) {
						if preKey, err := parseSliceKey(k, customKey); err != nil {
							return err
						} else {
							keyList[preKey] = struct{}{}
						}
					}
				}

				for k, _ := range keyList {
					v := reflect.Indirect(reflect.New(vField.Type().Elem()))
					if err = parser.bindValues(v, k, bind); err != nil {
						return
					}
					vField.Set(reflect.Append(vField, v))
				}
				goto NEXT
			}

			if tField.Type.Kind() == reflect.Slice {
				customKey += "[]"
			}

			item = parser.values[customKey]
			if len(item) > 0 {
				break
			}
		}
		if len(item) == 0 {
			if defaultValue := tField.Tag.Get("default"); defaultValue != "" {
				if err = setVal(vField, tField, defaultValue); err != nil {
					return errors.New("input <" + customKeystr + "> " + err.Error())
				}
			} else if bind == "required" {
				return errors.New("input <" + customKeystr + "> field is mismatch")
			}
			continue
		}

		if tField.Type.Kind() == reflect.Slice {
			for _, v := range item {
				if elem, err := appendElem(vField, tField, v); err != nil {
					return errors.New("input <" + customKeystr + "> " + err.Error())
				} else {
					vField.Set(elem)
				}
			}
		} else {
			if err = setVal(vField, tField, item[0]); err != nil {
				return errors.New("input <" + customKeystr + "> " + err.Error())
			}
		}
	NEXT:
	}

	return
}
