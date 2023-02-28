package play

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/leochen2038/play/logger"
)

type Action struct {
	name          string
	subsidiary    string
	metaData      map[string]string
	timeout       time.Duration
	instancesPool sync.Pool
	newHandle     func() interface{}
	input         map[string]ActionField
	output        map[string]ActionField
}

type ActionField struct {
	Field      string
	Keys       []string
	Tags       map[string]string
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

var ActionDefaultTimeout time.Duration = 500 * time.Millisecond
var actions = make(map[string]*Action, 32)

type Processor interface {
	Run(ctx *Context) (string, error)
}

func SetActionTimeout(name string, timeout time.Duration) {
	if v, ok := actions[name]; ok {
		v.timeout = timeout
	}
}

func NewProcessorWrap(handle interface{ Processor }, run func(p Processor, ctx *Context) (string, error), next map[string]*ProcessorWrap) *ProcessorWrap {
	return &ProcessorWrap{p: handle, run: run, next: next}
}

type ProcessorWrap struct {
	p    Processor
	run  func(p Processor, ctx *Context) (string, error)
	next map[string]*ProcessorWrap
}

func RegisterAction(name string, metaData map[string]string, new func() interface{}) {
	actions[name] = &Action{
		name:          name,
		metaData:      metaData,
		instancesPool: sync.Pool{New: new},
		newHandle:     new,
		input:         parseParameter(new().(*ProcessorWrap), "Input"),
		output:        parseParameter(new().(*ProcessorWrap), "Output"),
	}
}

func RunProcessor(s unsafe.Pointer, n uintptr, p Processor, ctx *Context) (string, error) {
	if n > 0 {
		var i uintptr
		ptr := uintptr(s)
		for i = 0; i < n; i++ {
			*(*byte)(unsafe.Pointer(ptr + i)) = 0
		}
		vInput := reflect.ValueOf(p).Elem().FieldByName("Input")
		if err := ctx.Input.Bind(vInput); err != nil {
			return "", err
		}
	}
	return p.Run(ctx)
}

// CallAction 消化其他错误，返回框架层面错误及其他panic
func CallAction(gctx context.Context, s *Session, request *Request) (err error) {
	var act *Action
	var timeout time.Duration = ActionDefaultTimeout
	hook := s.Server.Hook()

	if act = actions[request.ActionName]; act != nil && act.timeout > 0 {
		timeout = act.timeout
	}
	ctx := NewPlayContext(gctx, s, request, timeout)

	// defer func() {
	// 	if r := recover(); r != nil {
	// 		err = fmt.Errorf("panic: %v", r)
	// 	}
	// }()

	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			ctx.err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
		ctx.finish()
		go func() {
			defer func() {
				if panicInfo := recover(); panicInfo != nil {
					logger.System("panic on hook.OnFinish", "panicInfo", panicInfo, "stack", string(debug.Stack()))
				}
			}()
			hook.OnFinish(ctx)
			// ctx.gcfunc()
		}()
	}()

	if ctx.err = hook.OnRequest(ctx); ctx.Err() == nil {
		run(act, ctx)
	}

	if !ctx.ActionRequest.NonRespond {
		if hook.OnResponse(ctx); !ctx.ActionRequest.NonRespond {
			ctx.Response.Error = ctx.err
			if e := s.Write(&ctx.Response); e != nil {
				ctx.err = e
			}
		}
	}
	return
}

func run(act *Action, ctx *Context) {
	var flag string
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			ctx.err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
	}()
	if act == nil {
		ctx.err = errors.New("can not find action:" + ctx.ActionRequest.Name)
		return
	}

	ihandler := act.instancesPool.Get()
	if ihandler == nil {
		ctx.err = errors.New("can not get action handle from pool:" + ctx.ActionRequest.Name)
		return
	} else {
		defer act.instancesPool.Put(ihandler)
	}

	currentHandler := ihandler.(*ProcessorWrap)
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

func GetAction(name string) *Action {
	return actions[name]
}

func WalkAction(DoTask func(action *Action) error) error {
	var names []string
	for name := range actions {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if err := DoTask(actions[name]); err != nil {
			fmt.Println("DoTask error:", err)
			return err
		}
	}
	return nil
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
