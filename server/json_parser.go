package server

import (
	"errors"
	"github.com/tidwall/gjson"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type JsonParser struct {
	json gjson.Result
}

func NewJsonParser(data []byte) *JsonParser {
	parser := &JsonParser{json: gjson.GetBytes(data, "@this")}
	return parser
}

func (j *JsonParser) Bind(obj interface{}) (err error) {
	if vInput := reflect.ValueOf(obj).Elem().FieldByName("Input"); vInput.CanSet() {
		return j.bindGJson(vInput.Type(), vInput, &j.json)
	}
	return
}

func (j *JsonParser) GetVal(key string) (val interface{}, err error) {
	if result := j.json.Get(key); result.Exists() {
		val = result.Value()
	} else if key == "" {
		val = j.json.Value()
	} else {
		err = errors.New("can not find key|" + key)
	}

	return
}

func (j *JsonParser) bindGJson(t reflect.Type, v reflect.Value, source *gjson.Result) (err error) {
	var item gjson.Result
	var tField reflect.StructField
	var vField reflect.Value
	var regexPattern string
	var defaultValue string
	fieldCount := t.NumField()

	for i := 0; i < fieldCount; i++ {
		vField, tField = v.Field(i), t.Field(i)

		if !vField.CanInterface() {
			continue
		}

		if customKey := tField.Tag.Get("key"); customKey != "" {
			item = source.Get(customKey)
		} else {
			if item = source.Get(strings.ToLower(tField.Name[:1]) + tField.Name[1:]); !item.Exists() {
				item = source.Get(tField.Name)
			}
		}

		if !item.Exists() {
			if defaultValue = tField.Tag.Get("default"); defaultValue != "" {
				if tField.Type.Kind() == reflect.Slice {
					if elems, err := setSliceValueWithString(tField.Type.String(), vField, defaultValue); err != nil {
						return errors.New("input <" + tField.Name + "> " + err.Error())
					} else {
						vField.Set(elems)
					}
				} else {
					setValueWithString(&tField, vField, defaultValue)
				}
			} else if tField.Tag.Get("bind") == "required" {
				return errors.New("input <" + tField.Name + "> is required")
			}
			continue
		}

		if tField.Type.Kind().String() == "struct" && tField.Type.String() != "time.Time" {
			if err = j.bindGJson(tField.Type, vField.Elem(), &item); err != nil {
				return err
			}
			continue
		}

		if tField.Type.Kind().String() == "slice" {
			if tField.Type.Elem().Kind().String() == "struct" {
				item.ForEach(func(key, value gjson.Result) bool {
					if err = j.bindGJson(tField.Type.Elem(), vField.Elem(), &value); err != nil {
						return false
					}
					return true
				})
				if err != nil {
					return err
				}
			} else {
				elems := vField
				regexPattern = tField.Tag.Get("regex")
				item.ForEach(func(key, value gjson.Result) bool {
					if regexPattern != "" {
						if match, _ := regexp.MatchString(regexPattern, value.String()); match == false {
							if defaultValue = tField.Tag.Get("default"); defaultValue != "" {
								if elems, err := setSliceValueWithString(tField.Type.String(), vField, defaultValue); err != nil {
									err = errors.New("input <" + tField.Name + "> " + err.Error())
									return false
								} else {
									vField.Set(elems)
								}
							} else {
								err = errors.New("input <" + tField.Name + "> is mismatch")
							}
							return false
						}
					}
					if elems, err = setSliceValueWithGJson(tField.Type.String(), elems, &value); err != nil {
						return false
					}
					return true
				})
				if err != nil {
					return errors.New("input <" + tField.Name + "> " + err.Error())
				}
				vField.Set(elems)
			}
		} else {
			if regexPattern = tField.Tag.Get("regex"); regexPattern != "" {
				if match, _ := regexp.MatchString(regexPattern, item.String()); match == false {
					if defaultValue = tField.Tag.Get("default"); defaultValue != "" {
						if err = setValueWithString(&tField, vField, defaultValue); err != nil {
							return errors.New("input <" + tField.Name + "> " + err.Error())
						}
					} else {
						return errors.New("input <" + tField.Name + "> is mismatch")
					}
					continue
				}
			}
			if err = setValueWithGJson(&tField, vField, &item); err != nil {
				return errors.New("input <" + tField.Name + "> " + err.Error())
			}
		}
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

func setValueWithGJson(tField *reflect.StructField, vField reflect.Value, value *gjson.Result) error {
	fieldType := tField.Type.String()
	if fieldType == "interface {}" {
		vField.Set(reflect.ValueOf(value.Value()))
		return nil
	}
	if fieldType == "bool" {
		vField.SetBool(value.Bool())
		return nil
	}
	if fieldType != "time.Time" && fieldType != "string" && value.Type.String() != "Number" {
		if _, err := strconv.ParseFloat(value.Str, 64); err != nil {
			return errors.New("data type need number")
		}
	}
	switch fieldType {
	case "string":
		vField.SetString(value.String())
	case "int8":
		fallthrough
	case "int32":
		fallthrough
	case "int":
		fallthrough
	case "int64":
		vField.SetInt(value.Int())
	case "uint8":
		fallthrough
	case "uint32":
		fallthrough
	case "uint":
		fallthrough
	case "uint64":
		vField.SetUint(value.Uint())
	case "float":
		fallthrough
	case "float64":
		vField.SetFloat(value.Float())
	case "time.Time":
		if value.Type.String() != "Number" {
			layout := tField.Tag.Get("layout")
			if layout != "" {
				location := "Local"
				if zone := tField.Tag.Get("zone"); zone != "" {
					location = zone
				}
				local, err := time.LoadLocation(location)
				if err != nil {
					return err
				}
				if v, err := time.ParseInLocation(layout, value.String(), local); err != nil {
					return err
				} else {
					vField.Set(reflect.ValueOf(v))
				}
			} else {
				return errors.New("parser time error")
			}
		} else {
			v := time.Unix(value.Int(), 0)
			vField.Set(reflect.ValueOf(v))
		}
	}
	return nil
}
