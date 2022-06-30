package play

import (
	"errors"
	"reflect"
	"strings"
	"sync"

	"gitlab.youban.com/go-utils/play/codec/binders"
)

type Input struct {
	binder   binders.Binder
	exValues sync.Map
}

func NewInput(binder binders.Binder) Input {
	return Input{binder: binder}
}

func (input *Input) SetValue(key string, val interface{}) {
	input.exValues.Store(key, val)
}

func (input *Input) Value(key string) interface{} {
	if exValue, ok := input.exValues.Load(key); ok {
		return exValue
	} else {
		return input.binder.Get(key)
	}
}

func (input *Input) Bind(v reflect.Value) (err error) {
	if v.CanSet() {
		var tField reflect.StructField
		var vField reflect.Value
		var fieldCount = v.Type().NumField()

		for i := 0; i < fieldCount; i++ {
			if vField, tField = v.Field(i), v.Type().Field(i); !vField.CanInterface() {
				continue
			}

			key := tField.Tag.Get("key")
			if key == "" {
				key = tField.Name
			}
			for _, key := range strings.Split(key, ",") {
				if exValue, ok := input.exValues.Load(key); ok {
					if tField.Type.String() != reflect.TypeOf(exValue).String() {
						return errors.New("input custom " + key + " type need " + tField.Type.String() + " but " + reflect.TypeOf(exValue).String() + " given")
					}
					vField.Set(reflect.ValueOf(exValue))
					goto NEXT
				}
			}

			if err = input.binder.Bind(vField, tField); err != nil {
				return err
			}
		NEXT:
		}
	}
	return
}
