package play

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/leochen2038/play/codec/protos/golang/json"
)

type Action struct {
	name          string
	packageName   string
	metaData      map[string]string
	timeout       time.Duration
	instancesPool sync.Pool
	newHandle     func() interface{}
	input         map[string]ActionField
	output        map[string]ActionField
	example       string
}

type ActionField struct {
	Field      string
	Keys       []string
	Tags       map[string]string
	Sort       int
	Typ        string
	OriginType string
	Desc       string
	Required   bool
	Default    interface{}
	Child      map[string]ActionField
}

func (act *Action) Name() string {
	return act.name
}
func (act *Action) MetaData() map[string]string {
	return act.metaData
}
func (act *Action) Timeout() time.Duration {
	return act.timeout
}
func (act *Action) Instance() *ProcessorWrap {
	return act.newHandle().(*ProcessorWrap)
}
func (act *Action) Input() map[string]ActionField {
	return act.input
}
func (act *Action) Output() map[string]ActionField {
	return act.output
}
func (act *Action) Example() string {
	return act.example
}

var actions []*Action

type Processor interface {
	Run(ctx *Context) (string, error)
}

func NewProcessorWrap(handle interface{ Processor }, run func(p Processor, ctx *Context) (string, error), next map[string]*ProcessorWrap) *ProcessorWrap {
	return &ProcessorWrap{
		p:    handle,
		run:  run,
		next: next,
	}
}

type ProcessorWrap struct {
	p    Processor
	run  func(p Processor, ctx *Context) (string, error)
	next map[string]*ProcessorWrap
}

func RegisterAction(packageName, name string, metaData map[string]string, new func() interface{}) {
	actions = append(actions, &Action{
		name:          name,
		packageName:   packageName,
		metaData:      metaData,
		instancesPool: sync.Pool{New: new},
		newHandle:     new,
		input:         parseParameter(new().(*ProcessorWrap), "Input"),
		output:        parseParameter(new().(*ProcessorWrap), "Output"),
		example:       parseExample(new().(*ProcessorWrap), "Output"),
	})
}

func RunProcessor(s unsafe.Pointer, n uintptr, p Processor, ctx *Context) (string, error) {
	// if n > 0 {
	// 	var i uintptr
	// 	ptr := uintptr(s)
	// 	for i = 0; i < n; i++ {
	// 		*(*byte)(unsafe.Pointer(ptr + i)) = 0
	// 	}
	// }

	vInput := reflect.ValueOf(p).Elem().FieldByName("Input")
	if err := ctx.Input.Bind(vInput); err != nil {
		return "", err
	}
	return p.Run(ctx)
}

// DoRequest 消化其他错误，返回框架层面错误及其他panic
func DoRequest(gctx context.Context, s *Session, request *Request) (err error) {
	s.Server.Ctrl().AddTask()
	defer func() {
		s.Server.Ctrl().DoneTask()
	}()

	var ihandler interface{}
	actionTimeout := 500 * time.Millisecond
	actionExist := false
	actUnit := s.Server.LookupActionUnit(request.ActionName)
	if actUnit != nil {
		actionExist = true
		ihandler = actUnit.Action.newHandle()
		// ihandler = actUnit.Action.instancesPool.Get()
		// if ihandler == nil {
		// 	return errors.New("can not get action handle from pool:" + request.ActionName)
		// }
		actionTimeout = actUnit.Timeout
	}
	ctx := NewPlayContext(gctx, s, request, actionTimeout)
	ctx.ActionRequest.ActionExist = actionExist

	hook := ctx.Session.Server.Hook()

	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			ctx.err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
		ctx.Finish()
		go func() {
			defer func() {
				recover()
				// if ihandler != nil {
				// 	actUnit.Action.instancesPool.Put(ihandler)
				// }
			}()
			hook.OnFinish(ctx)
		}()
	}()

	if ctx.err = hook.OnRequest(ctx); ctx.Err() == nil && !ctx.isFinish {
		if !actionExist {
			ctx.err = errors.New("can not find action:" + ctx.ActionRequest.Name)
		} else {
			RunProcessorWrap(ihandler.(*ProcessorWrap), ctx)
		}
	}

	if !ctx.ActionRequest.NonRespond {
		if hook.OnResponse(ctx); !ctx.ActionRequest.NonRespond {
			ctx.Response.Error = ctx.err
			if e := ctx.Session.Write(&ctx.Response); e != nil {
				ctx.err = e
			}
		}
	}

	return
}

func RunProcessorWrap(currentHandler *ProcessorWrap, ctx *Context) {
	var flag string
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			ctx.err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
	}()

	for ok := true; ok; currentHandler, ok = currentHandler.next[flag] {
		flag, ctx.err = currentHandler.run(currentHandler.p, ctx)
		if ctx.Err() != nil {
			return
		}

		if procOutputType, ok := reflect.TypeOf(currentHandler.p).Elem().FieldByName("Output"); ok {
			procOutputVal := reflect.ValueOf(currentHandler.p).Elem().FieldByName("Output")
			for i := 0; i < procOutputType.Type.NumField(); i++ {
				if structValue := procOutputVal.Field(i); structValue.CanSet() {
					structType := procOutputType.Type.Field(i)
					structKey := structType.Tag.Get("key")
					if structKey == "" {
						structKey = structType.Name
					}
					ctx.Response.Output.Set(structKey, structValue.Interface())
				}
			}
		}
	}
}

func ActionsByPackage(packageName string) []*Action {
	var result []*Action
	for _, action := range actions {
		if action.packageName == packageName {
			result = append(result, action)
		}
	}
	return result
}

func parseParameter(handle *ProcessorWrap, field string) map[string]ActionField {
	value := reflect.ValueOf(handle.p).Elem().FieldByName(field)
	fields := parserField(value)

	var nextFields = make([]map[string]ActionField, 0)
	for _, next := range handle.next {
		nextFields = append(nextFields, parseParameter(next, field))
	}

	for _, nFields := range nextFields {
		for k, f := range nFields {
			if nextAllFind(k, nextFields) && (field == "Output" || f.Required) {
				f.Required = true
			} else {
				f.Required = false
			}
			changeRequired(&f, f.Required)
			if _, ok := fields[k]; !ok {
				fields[k] = f
			}
		}
	}

	return fields
}

func changeRequired(field *ActionField, required bool) {
	for name, child := range field.Child {
		f := field.Child[name]
		f.Required = required
		field.Child[name] = f
		changeRequired(&child, required)
	}
}

func nextAllFind(fieldName string, nextFields []map[string]ActionField) bool {
	for _, next := range nextFields {
		if _, ok := next[fieldName]; !ok {
			return false
		}
	}
	return true
}

func parserField(value reflect.Value) map[string]ActionField {
	if !value.IsValid() {
		return nil
	}

	fields := make(map[string]ActionField)

	for i := 0; i < value.NumField(); i++ {
		field := ActionField{}

		structType := value.Type().Field(i)
		structKey := structType.Tag.Get("key")
		structNote := structType.Tag.Get("note")
		structRequire := structType.Tag.Get("required")
		structDefault := structType.Tag.Get("default")
		if len(structKey) > 0 {
			field.Keys = strings.Split(structKey, ",")
		}
		if len(structNote) == 0 {
			structNote = structType.Tag.Get("label")
		}

		field.Sort = i
		field.Field = structType.Name
		field.Tags = TagLookup(string(structType.Tag))
		field.Typ = structType.Type.String()
		field.OriginType = structType.Type.String()
		field.Desc = structNote
		field.Required = structRequire == "true"
		field.Default = structDefault

		switch structType.Type.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64, reflect.String, reflect.Interface, reflect.Bool:

			fields[field.Field] = field
		case reflect.Array, reflect.Slice:
			elemType := reflect.New(structType.Type.Elem())
			elemValue := reflect.Indirect(elemType)

			if elemValue.Type().Kind() == reflect.Struct {
				if strings.Contains(field.OriginType, "[]struct") {
					field.OriginType = "[]" + field.Field
				}
				field.Typ = "[]object"
				field.Child = parserField(elemValue)
			}

			if elemValue.Type().Kind() == reflect.Uint8 {
				field.Typ = "[]byte"
			}

			fields[field.Field] = field
		case reflect.Uint8:
			field.Typ = "byte"

			fields[field.Field] = field
		case reflect.Map:
			field.Typ = "map"
			elemType := reflect.New(structType.Type.Elem())
			elemValue := reflect.Indirect(elemType)

			if elemValue.Type().Kind() == reflect.Struct {
				field.Child = parserField(elemValue)
			}

			if elemValue.Type().Kind() == reflect.Map {
				lastIndex := strings.LastIndex(field.OriginType, "]")
				field.OriginType = field.OriginType[:lastIndex+1] + "interface {}"
			}

			fields[field.Field] = field
		case reflect.Struct:
			if strings.Contains(field.OriginType, "struct") {
				field.OriginType = field.Field
			}

			field.Child = parserField(value.Field(i))
			field.Typ = "object"

			fields[field.Field] = field
		}
	}

	return fields
}

func TagLookup(tag string) (res map[string]string) {
	res = make(map[string]string)
	for tag != "" {
		// Skip leading space.
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		name := string(tag[:i])
		tag = tag[i+1:]

		// Scan quoted string to find value.
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}
		qvalue := string(tag[:i+1])
		tag = tag[i+1:]
		res[name], _ = strconv.Unquote(qvalue)
	}
	return
}

func parseExample(handle *ProcessorWrap, field string) string {
	data := fillExample(handle, field)
	bytes, _ := json.MarshalIndent(data, "", "    ")
	return string(bytes)
}

func fillExample(handle *ProcessorWrap, field string) map[string]interface{} {
	data := make(map[string]interface{}, 10)

	v := reflect.ValueOf(handle.p).Elem().FieldByName(field)
	if v.CanSet() {
		var tField reflect.StructField
		var vField reflect.Value
		var fieldCount = v.Type().NumField()
		for i := 0; i < fieldCount; i++ {
			if vField, tField = v.Field(i), v.Type().Field(i); !vField.CanInterface() {
				continue
			}

			switch tField.Type.Kind() {
			case reflect.Struct:
				fillStruct(vField)
			case reflect.Slice:
				fillSlice(vField)
			case reflect.Map:
				fillMap(vField)
			case reflect.Ptr:
				fillPtr(vField)
			}

			key := tField.Tag.Get("key")
			if key == "" {
				key = tField.Name
			}
			data[key] = vField.Interface()
		}
	}

	for _, next := range handle.next {
		for key, val := range fillExample(next, field) {
			data[key] = val
		}
	}
	return data
}

func fillStruct(v reflect.Value) {
	if v.CanSet() {
		var tField reflect.StructField
		var vField reflect.Value

		var fieldCount = v.Type().NumField()
		for i := 0; i < fieldCount; i++ {
			if vField, tField = v.Field(i), v.Type().Field(i); !vField.CanInterface() {
				continue
			}

			switch tField.Type.Kind() {
			case reflect.Struct:
				fillStruct(vField)
			case reflect.Slice:
				fillSlice(vField)
			case reflect.Map:
				fillMap(vField)
			case reflect.Ptr:
				fillPtr(vField)
			}
		}
	}
}

func fillSlice(vField reflect.Value) {
	if vField.Type().Elem().Kind() == reflect.Struct {
		v := reflect.Indirect(reflect.New(vField.Type().Elem()))
		fillStruct(v)
		vField.Set(reflect.Append(vField, v))
	} else if vField.Type().Elem().Kind() == reflect.Map {
		v := reflect.MakeMap(vField.Type().Elem())
		vField.Set(reflect.Append(vField, v))
	} else if vField.Type().Elem().Kind() == reflect.Ptr {
		v := reflect.Indirect(reflect.New(vField.Type().Elem()))
		fillPtr(v)
		vField.Set(reflect.Append(vField, v))
	} else {
		v := reflect.Indirect(reflect.New(vField.Type().Elem()))
		vField.Set(reflect.Append(vField, v))
	}
}

func fillMap(vField reflect.Value) {
	vField.Set(reflect.MakeMap(vField.Type()))
}

func fillPtr(vField reflect.Value) {
	if vField.Type().Elem().Kind() == reflect.Struct {
		vField.Set(reflect.New(vField.Type().Elem()))
		v := reflect.Indirect(reflect.New(vField.Type().Elem()))
		fillStruct(v)
		vField.Elem().Set(v)
	}
}
