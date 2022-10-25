package binders

import (
	"errors"
	"reflect"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type ProtobufBinder struct {
	message protoreflect.Message
}

func GetBinderOfProtobuf(desc protoreflect.Message) Binder {
	return &ProtobufBinder{message: desc}
}

func (b ProtobufBinder) Name() string {
	return "protobuf"
}

func (b ProtobufBinder) Bind(v reflect.Value, s reflect.StructField) error {
	return b.bindProtobuf(v, s, b.message, "")
}

func (b ProtobufBinder) Get(key string) (val interface{}) {
	if key == "" {
		return nil
	}
	if item := b.message.Type().Descriptor().Fields().ByName(protoreflect.Name(key)); item == nil {
		return nil
	} else {
		return b.message.Get(item).Interface()
	}
}

func (b ProtobufBinder) bindProtobuf(v reflect.Value, s reflect.StructField, source protoreflect.Message, preKey string) (err error) {
	var customKey, required, fullKey string
	var item protoreflect.FieldDescriptor
	var messageFields = source.Type().Descriptor().Fields()

	required, customKey = s.Tag.Get("required"), s.Tag.Get("key")
	for _, v := range strings.Split(customKey, ",") {
		if preKey != "" {
			fullKey = preKey + "." + v
		} else {
			fullKey = v
		}

		item = messageFields.ByName(protoreflect.Name(v))
		if item != nil && source.Has(item) {
			break
		}
	}
	if item == nil || !source.Has(item) {
		if defaultValue := s.Tag.Get("default"); defaultValue != "" {
			if err = setProtobufVal(v, s, defaultValue, nil); err != nil {
				return errors.New("input: " + customKey + " <" + s.Tag.Get("note") + "> " + err.Error())
			}
		} else if required == "true" {
			return errors.New("input: " + fullKey + " <" + s.Tag.Get("note") + "> field is mismatch 1")
		}
		return nil
	}

	switch s.Type.Kind() {
	case reflect.Struct:
		if s.Type.String() == "time.Time" {
			return setProtobufVal(v, s, source.Get(item).String(), nil)
		} else {
			return b.bindStruct(v, dynamicpb.NewMessage(item.Message()), fullKey)
		}
	case reflect.Slice:
		if s.Type.String() == "[]uint8" {
			return b.bindBytes(v, s, source.Get(item).Bytes(), fullKey)
		} else {
			return b.bindSlice(v, s, source.Get(item).List(), fullKey)
		}
	default:
		vv := source.Get(item)
		return setProtobufVal(v, s, source.Get(item).String(), &vv)
	}
}

func (b ProtobufBinder) bindStruct(v reflect.Value, source protoreflect.Message, preKey string) error {
	count := v.Type().NumField()
	for i := 0; i < count; i++ {
		if err := b.bindProtobuf(v.Field(i), v.Type().Field(i), source, preKey); err != nil {
			return err
		}
	}
	return nil
}

func (b ProtobufBinder) bindBytes(vField reflect.Value, s reflect.StructField, bytes []byte, preKey string) (err error) {
	vField.Set(reflect.ValueOf(bytes))
	return
}

func (b ProtobufBinder) bindSlice(vField reflect.Value, s reflect.StructField, iList protoreflect.List, preKey string) (err error) {
	fieldKind := vField.Type().Elem().Kind()
	if fieldKind == reflect.Struct {
		for i := 0; i < iList.Len(); i++ {
			v := reflect.Indirect(reflect.New(vField.Type().Elem()))
			if err = b.bindStruct(v, iList.Get(i).Message(), preKey); err != nil {
				return errors.New("input: " + preKey + " <" + s.Tag.Get("note") + ">  type list is mismatch")
			}
			vField.Set(reflect.Append(vField, v))
		}
	} else {
		var elems = vField
		for i := 0; i < iList.Len(); i++ {
			v := iList.Get(i)
			if elems, err = setSliceValueWithProtobuf(vField.Type().String(), elems, &v); err != nil {
				return errors.New("input: " + preKey + " <" + s.Tag.Get("note") + "> " + err.Error())
			}
			vField.Set(elems)
		}
	}
	return
}

func setProtobufVal(vField reflect.Value, tField reflect.StructField, str string, pValue *protoreflect.Value) error {
	var err error
	var val interface{}

	if err = checkRegex(tField, str); err != nil {
		return err
	}

	if tStr := strings.Trim(tField.Type.String(), "[]"); tStr == "interface {}" {
		val = pValue.Interface()
	} else {
		if val, err = parseInterface(tStr, pValue.String(), tField); err != nil {
			return err
		}
	}

	vField.Set(reflect.ValueOf(val))
	return nil
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
