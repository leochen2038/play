package parsers

import (
	"errors"
	"net/url"
	"reflect"
	"strings"
)

type FormDataParser struct {
	values map[string][]string
}

func NewFormDataParser(values url.Values) *FormDataParser {
	parser := &FormDataParser{values: values}
	return parser
}

func (parser *FormDataParser)GetVal(key string) (interface{}, error) {
	return nil, nil
}

func (parser *FormDataParser) Bind(obj interface{}) error {
	if vInput := reflect.ValueOf(obj).Elem().FieldByName("Input"); vInput.CanSet() {
		return parser.bindValues(vInput, "", "")
	}
	return nil
}

func (parser *FormDataParser) bindValues(v reflect.Value, prefix string, required string) (err error) {
	var tField reflect.StructField
	var vField reflect.Value
	var item []string
	var fieldCount = v.Type().NumField()
	var customKey string
	var bind string // required, optional

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
		if prefix != "" {
			customKey = prefix + "[" + customKey + "]"
		}

		if tField.Type.Kind() == reflect.Struct && tField.Type.String() != "time.Time" {
			if err = parser.bindValues(vField, customKey, bind); err != nil {
				return
			}
			continue
		}
		if tField.Type.Kind() == reflect.Slice && vField.Type().Elem().Kind() == reflect.Struct && vField.Type().Elem().String() != "time.Time" {
			var keyList =make(map[string]struct{}, 8)
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
			continue
		}

		if tField.Type.Kind() == reflect.Slice {
			customKey += "[]"
		}

		item = parser.values[customKey]
		if len(item) == 0 {
			if bind == "required" {
				if defaultValue := tField.Tag.Get("default"); defaultValue != "" {
					if err = setVal(vField, tField, defaultValue); err != nil {
						return errors.New("input <" + customKey + "> " + err.Error())
					}
				} else {
					return errors.New("input <" + customKey + "> field is mismatch")
				}
			}
			continue
		}

		if tField.Type.Kind() == reflect.Slice {
			for _, v := range item {
				if elem, err := appendElem(vField, tField, v); err != nil {
					return errors.New("input <" + customKey + "> " + err.Error())
				} else {
					vField.Set(elem)
				}
			}
		} else {
			if err = setVal(vField, tField, item[0]); err != nil {
				return errors.New("input <" + customKey + "> " + err.Error())
			}
		}
	}

	return
}