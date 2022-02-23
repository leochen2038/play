package binder

import (
	"errors"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type ProtobufBinder struct {
	message protoreflect.Message
	exData  map[string]interface{}
}

func NewProtobufBinder(desc protoreflect.Message) *ProtobufBinder {
	parser := &ProtobufBinder{message: desc, exData: make(map[string]interface{})}
	return parser
}

func (j *ProtobufBinder) Bind(v reflect.Value) error {
	if v.CanSet() {
		return j.bindProtobuf(v, j.message, "", "")
	}
	return nil
}

func (j *ProtobufBinder) Get(key string) (val interface{}, err error) {
	if key == "" {
		return nil, errors.New("unsuport empty key")
	}
	if item := j.message.Type().Descriptor().Fields().ByName(protoreflect.Name(key)); item == nil {
		return nil, errors.New("can not find key|" + key)
	} else {
		return j.message.Get(item).Interface(), nil
	}
}

func (j *ProtobufBinder) Set(key string, val interface{}) {
	j.exData[key] = val
}

func (j *ProtobufBinder) bindProtobuf(v reflect.Value, source protoreflect.Message, required string, preKey string) (err error) {
	var tField reflect.StructField
	var vField reflect.Value
	var item protoreflect.FieldDescriptor
	var fieldCount = v.Type().NumField()
	var customKey string
	var bind string // required, optional
	var fullKey string
	var messageFields = source.Type().Descriptor().Fields()

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

			item = messageFields.ByName(protoreflect.Name(v))
			if item != nil && source.Has(item) {
				break
			}
		}
		if item == nil {
			if defaultValue := tField.Tag.Get("default"); defaultValue != "" {
				if err = setProtobufVal(vField, tField, defaultValue, nil); err != nil {
					return errors.New("input <" + customKey + "> " + err.Error())
				}
			} else if bind == "required" {
				return errors.New("input <" + fullKey + "> field is mismatch 1")
			}
			continue
		}
		if !source.Has(item) {
			if defaultValue := tField.Tag.Get("default"); defaultValue != "" {
				if err = setProtobufVal(vField, tField, defaultValue, nil); err != nil {
					return errors.New("input <" + customKey + "> " + err.Error())
				}
				continue
			}
		}
		if tField.Type.Kind() == reflect.Struct && tField.Type.String() != "time.Time" {
			if err = j.bindProtobuf(vField, dynamicpb.NewMessage(item.Message()), bind, fullKey); err != nil {
				return
			}
			continue
		}
		if tField.Type.Kind() == reflect.Slice && vField.Type().Elem().Kind() == reflect.Struct && vField.Type().Elem().String() != "time.Time" {
			var count int
			iList := source.Get(item).List()
			for i := 0; i < iList.Len(); i++ {
				v := reflect.Indirect(reflect.New(vField.Type().Elem()))
				if err = j.bindProtobuf(v, iList.Get(i).Message(), bind, fullKey); err != nil {
					return errors.New("input <" + fullKey + ">  type list is mismatch")
				}
				vField.Set(reflect.Append(vField, v))
			}
			if err != nil {
				return err
			}
			if count == 0 && bind == "required" {
				return errors.New("input <" + fullKey + "> field is mismatch 2")
			}
			continue
		}

		if tField.Type.Kind() == reflect.Slice {
			var count int
			var elems = vField
			iList := source.Get(item).List()
			for i := 0; i < iList.Len(); i++ {
				count++
				v := iList.Get(i)
				if elems, err = setSliceValueWithProtobuf(vField.Type().String(), elems, &v); err != nil {
					return errors.New("input <" + fullKey + "> " + err.Error())
				}
				vField.Set(elems)
			}
			if count == 0 && bind == "required" {
				return errors.New("input <" + fullKey + "> field is mismatch 3")
			}
		} else {
			v := source.Get(item)
			if err = setProtobufVal(vField, tField, v.String(), &v); err != nil {
				return errors.New("input <" + fullKey + "> " + err.Error())
			}
		}
	NEXT:
	}
	return
}

func setSliceValueWithProtobuf(fieldType string, elems reflect.Value, value *protoreflect.Value) (reflect.Value, error) {
	if fieldType == "[]interface {}" {
		elems = reflect.Append(elems, reflect.ValueOf(value.Interface()))
		return elems, nil
	}
	//if fieldType != "[]string" && value.Type.String() != "Number" {
	//	if _, err := strconv.ParseFloat(value.Str, 64); err != nil {
	//		return elems, errors.New("data type need number")
	//	}
	//}
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

func setProtobufVal(vField reflect.Value, tField reflect.StructField, str string, gValue *protoreflect.Value) error {
	if regexPattern := tField.Tag.Get("regex"); regexPattern != "" {
		if match, _ := regexp.MatchString(regexPattern, str); match == false {
			return errors.New("value is mismatch")
		}
	}

	if val, err := parseProtobuf(tField, str, gValue); err != nil {
		return err
	} else {
		vField.Set(reflect.ValueOf(val))
	}
	return nil
}

func parseProtobuf(tField reflect.StructField, str string, gValue *protoreflect.Value) (interface{}, error) {
	switch strings.Trim(tField.Type.String(), "[]") {
	case "interface {}":
		if gValue == nil {
			return str, nil
		}
		return gValue.Interface(), nil
	case "string":
		return str, nil
	case "time.Time":
		return parseTime(tField, str)
	case "bool":
		return strconv.ParseBool(str)
	case "byte":
		return parseByte(str, 10)
	case "int":
		return strconv.Atoi(str)
	case "int8":
		return parseInt8(str, 10)
	case "int16":
		return parseInt16(str, 10)
	case "int32":
		return parseInt32(str, 10)
	case "int64":
		return parseInt64(str, 10)
	case "uint":
		return parseUint(str, 10)
	case "uint8":
		return parseUint8(str, 10)
	case "uint16":
		return parseUint16(str, 10)
	case "uint32":
		return parseUint32(str, 10)
	case "uint64":
		return parseUint64(str, 10)
	case "float32":
		return parseFloat32(str)
	case "float64":
		return strconv.ParseFloat(str, 64)
	}
	return nil, errors.New("not supported type " + tField.Type.String())
}
