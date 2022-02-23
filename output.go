package play

import (
	"errors"
	"fmt"
	"github.com/leochen2038/play/library/golang/json"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"reflect"
)

type Output interface {
	Get(key string) interface{}
	Set(key string, val interface{})
	ToJsonRaw() ([]byte, error)
	ToProtobuf(descriptor protoreflect.MessageDescriptor) ([]byte, error)
	All() interface{}
}

type KvOutput struct {
	data map[string]interface{}
}

func (o *KvOutput) Get(key string) interface{} {
	if key != "" {
		val, _ := o.data[key]
		return val
	}
	return o.data
}

func (o *KvOutput) All() interface{} {
	return o.data
}

func (o *KvOutput) Set(key string, val interface{}) {
	if o.data == nil {
		o.data = make(map[string]interface{}, 10)
	}
	o.data[key] = val
}

func (o *KvOutput) ToJsonRaw() (data []byte, err error) {
	return json.MarshalEscape(o.data, false, false)
}

func (o *KvOutput) ToProtobuf(descriptor protoreflect.MessageDescriptor) (data []byte, err error) {
	if message, err := _toProtobuf(o.data, descriptor); err != nil {
		return nil, err
	} else {
		return proto.Marshal(message)
	}
}

func _toProtobufRef(data reflect.Value, descriptor protoreflect.MessageDescriptor) (proto.Message, error) {
	message := dynamicpb.NewMessage(descriptor)
	structFieldNum := data.Type().NumField()

	for i := 0; i < structFieldNum; i++ {
		customKey := data.Type().Field(i).Tag.Get("key")
		val := data.Field(i)
		if item := descriptor.Fields().ByName(protoreflect.Name(customKey)); item != nil {
			if item.IsList() {
				if val.Type().Kind() != reflect.Slice {
					return nil, errors.New("assigning " + customKey + " invalid type " + reflect.TypeOf(val).Kind().String() + " need slice")
				}
				lst := message.NewField(item).List()
				for i := 0; i < val.Len(); i++ {
					if pbVal, err := _convertProtobufVal(item, val.Index(i).Interface()); err != nil {
						return nil, err
					} else {
						lst.Append(pbVal)
					}
				}
				message.Set(item, protoreflect.ValueOf(lst))
			} else {
				if item.Kind().String() == "message" {
					if sub, err := _toProtobufRef(val, item.Message()); err != nil {
						return nil, err
					} else {
						message.Set(item, protoreflect.ValueOfMessage(sub.ProtoReflect()))
					}
				} else {
					message.Set(item, protoreflect.ValueOf(val.Interface()))
				}
			}
		}
	}

	return message, nil
}

func _toProtobuf(data map[string]interface{}, descriptor protoreflect.MessageDescriptor) (proto.Message, error) {
	message := dynamicpb.NewMessage(descriptor)
	for i := 0; i < descriptor.Fields().Len(); i++ {
		field := descriptor.Fields().Get(i)
		var key = string(descriptor.Fields().Get(i).Name())
		if val, ok := data[key]; ok {
			if field.IsList() {
				if reflect.TypeOf(val).Kind() != reflect.Slice {
					return nil, errors.New("assigning " + key + " invalid type string need slice")
				}
				lst := message.NewField(field).List()
				vRef := reflect.ValueOf(val)
				for i := 0; i < vRef.Len(); i++ {
					if pbVal, err := _convertProtobufVal(field, vRef.Index(i).Interface()); err != nil {
						return nil, err
					} else {
						lst.Append(pbVal)
					}
				}
				message.Set(field, protoreflect.ValueOfList(lst))
			} else {
				if pbVal, err := _convertProtobufVal(field, val); err != nil {
					return nil, err
				} else {
					message.Set(field, pbVal)
				}
			}
		}
	}
	return message, nil
}

func _convertProtobufVal(field protoreflect.FieldDescriptor, val interface{}) (pbVal protoreflect.Value, err error) {
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = fmt.Errorf("%v", panicInfo)
		}
	}()
	tRef := reflect.TypeOf(val)
	if tRef.Kind() == reflect.Struct {
		var sub proto.Message
		if sub, err = _toProtobufRef(reflect.ValueOf(val), field.Message()); err != nil {
			return
		}
		return protoreflect.ValueOfMessage(sub.ProtoReflect()), nil
	}
	switch v := val.(type) {
	case map[string]interface{}:
		var sub proto.Message
		if sub, err = _toProtobuf(v, field.Message()); err != nil {
			return
		} else {
			return protoreflect.ValueOfMessage(sub.ProtoReflect()), nil
		}
	default:
		return protoreflect.ValueOf(val), nil
	}
}
