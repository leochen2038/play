package server

import (
	"errors"
	"github.com/leochen2038/play"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type HttpParser struct {
	values map[string][]string
}

func NewHttpParser(request *http.Request) play.Parser {
	contentType := request.Header.Get("Content-Type")

	if strings.Contains(contentType, "json") {
		raw, _ := ioutil.ReadAll(request.Body)
		return &JsonParser{json: gjson.GetBytes(raw, "@this")}
	}

	if strings.Contains(contentType, "form-urlencoded") {
		_ = request.ParseForm()
		return &HttpParser{values: request.Form}
	}

	if strings.Contains(contentType, "multipart/form-data") {
		var maxMemory int64 = 1024 * 1024 * 10 // 10m
		_ = request.ParseMultipartForm(maxMemory)
		return &HttpParser{values: request.Form}
	}

	return &HttpParser{values: request.URL.Query()}
}

func (i *HttpParser) GetVal(key string) (interface{}, error) {
	var val []string
	var ok bool
	if val, ok = i.values[key]; !ok {
		return nil, errors.New("can not find key " + key)
	} else if key == "" {
		values := map[string]interface{}{}
		for k, v := range i.values {
			if len(val) != 0 {
				values[k] = v[0]
			}
		}
	}

	if len(val) == 0 {
		return nil, errors.New("can not find key " + key)
	}
	return val[0], nil
}

func (h *HttpParser) Bind(obj interface{}) (err error) {
	return h.bindHttpValues(reflect.TypeOf(obj).Elem(), reflect.ValueOf(obj).Elem())
}

func (h *HttpParser) bindHttpValues(t reflect.Type, v reflect.Value) (err error) {
	var tField reflect.StructField
	var vField reflect.Value
	var regexPattern string
	var defaultValue string
	var item []string
	var subfix string
	var fieldCount = t.NumField()

	for i := 0; i < fieldCount; i++ {
		item, subfix = nil, ""
		vField, tField = v.Field(i), t.Field(i)

		if tField.Type.Kind() == reflect.Struct {
			return errors.New("not support struct")
		}

		if tField.Type.Kind() == reflect.Slice {
			elKind := tField.Type.Elem().Kind()
			if elKind == reflect.Slice || elKind == reflect.Map || elKind == reflect.Struct {
				return errors.New("not support slice of struct, map, slice")
			}
			subfix = "[]"
		}

		if customKey := tField.Tag.Get("key"); customKey != "" {
			item, _ = h.values[customKey]
		} else {
			if tField.Type.Kind() == reflect.Slice {
				subfix = "[]"
			}
			if item, _ = h.values[strings.ToLower(tField.Name[:1])+tField.Name[1:]+subfix]; len(item) == 0 {
				item = h.values[tField.Name+subfix]
			}
		}

		if len(item) == 0 {
			if defaultValue = tField.Tag.Get("default"); defaultValue != "" {
				if tField.Type.Kind() == reflect.Slice {
					if elems, err := setSliceValueWithString(tField.Type.String(), vField, defaultValue); err != nil {
						return errors.New("input <" + tField.Name + "> " + err.Error())
					} else {
						vField.Set(elems)
					}
				} else {
					if err := setValueWithString(tField.Type.String(), vField, defaultValue); err != nil {
						return errors.New("input <" + tField.Name + "> " + err.Error())
					}
				}
			} else if tField.Tag.Get("bind") == "required" {
				return errors.New("input <" + tField.Name + "> is required")
			}
			continue
		}

		if tField.Type.Kind() == reflect.Slice {
			elems := vField
			for _, value := range item {
				if regexPattern = tField.Tag.Get("regex"); regexPattern != "" {
					if match, _ := regexp.MatchString(regexPattern, value); match == false {
						if defaultValue = tField.Tag.Get("default"); defaultValue != "" {
							if elems, err := setSliceValueWithString(tField.Type.String(), vField, defaultValue); err != nil {
								return errors.New("input <" + tField.Name + "> " + err.Error())
							} else {
								vField.Set(elems)
							}
						} else {
							return errors.New("input <" + tField.Name + "> is mismatch")
						}
						break
					}
				}
				if elems, err = setSliceValueWithString(tField.Type.String(), elems, value); err != nil {
					return errors.New("input <" + tField.Name + "> " + err.Error())
				}
			}
			vField.Set(elems)
		} else {
			if regexPattern = tField.Tag.Get("regex"); regexPattern != "" {
				if match, _ := regexp.MatchString(regexPattern, item[0]); match == false {
					if defaultValue = tField.Tag.Get("default"); defaultValue != "" {
						if err = setValueWithString(tField.Type.String(), vField, defaultValue); err != nil {
							return errors.New("input <" + tField.Name + "> " + err.Error())
						}
					} else {
						return errors.New("input <" + tField.Name + "> is mismatch")
					}
					continue
				}
			}
			if err = setValueWithString(tField.Type.String(), vField, item[0]); err != nil {
				return errors.New("input <" + tField.Name + "> " + err.Error())
			}
		}
	}

	return
}

func setSliceValueWithString(fieldType string, elems reflect.Value, value string) (reflect.Value, error) {
	switch fieldType {
	case "[]interface {}":
		elems = reflect.Append(elems, reflect.ValueOf(value))
	case "[]string":
		elems = reflect.Append(elems, reflect.ValueOf(value))
	case "[]int8":
		if n, err := strconv.ParseInt(value, 10, 8); err != nil {
			return elems, err
		} else {
			elems = reflect.Append(elems, reflect.ValueOf(int8(n)))
		}
	case "[]uint8":
		if n, err := strconv.ParseUint(value, 10, 8); err != nil {
			return elems, err
		} else {
			elems = reflect.Append(elems, reflect.ValueOf(uint(n)))
		}
	case "[]int32":
		if n, err := strconv.ParseInt(value, 10, 32); err != nil {
			return elems, err
		} else {
			elems = reflect.Append(elems, reflect.ValueOf(int32(n)))
		}
	case "[]uint32":
		if n, err := strconv.ParseUint(value, 10, 32); err != nil {
			return elems, err
		} else {
			elems = reflect.Append(elems, reflect.ValueOf(uint32(n)))
		}
	case "[]int64":
		if n, err := strconv.ParseInt(value, 10, 64); err != nil {
			return elems, err
		} else {
			elems = reflect.Append(elems, reflect.ValueOf(int64(n)))
		}
	case "[]uint64":
		if n, err := strconv.ParseUint(value, 10, 64); err != nil {
			return elems, err
		} else {
			elems = reflect.Append(elems, reflect.ValueOf(uint64(n)))
		}
	case "[]int":
		if n, err := strconv.ParseInt(value, 10, 32); err != nil {
			return elems, err
		} else {
			elems = reflect.Append(elems, reflect.ValueOf(int(n)))
		}
	case "[]uint":
		if n, err := strconv.ParseUint(value, 10, 32); err != nil {
			return elems, err
		} else {
			elems = reflect.Append(elems, reflect.ValueOf(uint(n)))
		}
	case "[]float32":
		if n, err := strconv.ParseFloat(value, 32); err != nil {
			return elems, err
		} else {
			elems = reflect.Append(elems, reflect.ValueOf(float32(n)))
		}
	case "[]float64":
		if n, err := strconv.ParseFloat(value, 64); err != nil {
			return elems, err
		} else {
			elems = reflect.Append(elems, reflect.ValueOf(n))
		}
	}

	return elems, nil
}

func setValueWithString(fieldType string, vField reflect.Value, value string) error {
	switch fieldType {
	case "interface {}":
		vField.Set(reflect.ValueOf(value))
	case "string":
		vField.SetString(value)
	case "int8":
		fallthrough
	case "int32":
		fallthrough
	case "int64":
		fallthrough
	case "int":
		if n, err := strconv.ParseInt(value, 10, 64); err != nil {
			return err
		} else {
			vField.SetInt(n)
		}
	case "uint8":
		fallthrough
	case "uint32":
		fallthrough
	case "uint64":
		fallthrough
	case "uint":
		if n, err := strconv.ParseUint(value, 10, 64); err != nil {
			return err
		} else {
			vField.SetUint(n)
		}
	case "float32":
		fallthrough
	case "float64":
		if n, err := strconv.ParseFloat(value, 64); err != nil {
			return err
		} else {
			vField.SetFloat(n)
		}
	}
	return nil
}
