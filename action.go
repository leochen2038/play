package play

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"time"
	"unsafe"
)

type action struct {
	subsidiary    string
	metaData      map[string]string
	timeout       int32
	instancesPool sync.Pool
	newHandle     func() interface{}
}

func (act action) MetaData() map[string]string {
	return act.metaData
}
func (act action) Timeout() int32 {
	return act.timeout
}
func (act action) Instance() *ProcessorWrap {
	return act.newHandle().(*ProcessorWrap)
}

var ActionDefaultTimeout int32 = 500
var actions = make(map[string]*action, 32)

type Processor interface {
	Run(ctx *Context) (string, error)
}

func SetActionTimeout(name string, timeout int32) {
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

func RegisterAction(name string, metaData map[string]string, new func() interface{}, timeout int32) {
	actions[name] = &action{metaData: metaData, timeout: timeout, instancesPool: sync.Pool{New: new}}
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

// CallAction 消化其他错误，只返回onFinish错误
func CallAction(gctx context.Context, s *Session, request *Request) (err error) {
	var act *action
	var timeout int32 = ActionDefaultTimeout
	hook := s.Server.Hook()

	if act = actions[request.ActionName]; act != nil && act.timeout > 0 {
		timeout = act.timeout
	}
	ctx := NewPlayContext(gctx, s, request, time.Duration(timeout)*time.Millisecond)

	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			ctx.err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
		ctx.gcfunc()
		err = hook.OnFinish(ctx)
	}()

	if ctx.err = hook.OnRequest(ctx); ctx.Err() == nil {
		run(act, ctx)
	}

	if ctx.err = hook.OnResponse(ctx); ctx.Err() == nil {
		if request.Respond {
			ctx.err = s.Write(&ctx.Response)
		}
	}
	return
}

func run(act *action, ctx *Context) {
	var flag string
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			ctx.err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
	}()
	if act == nil {
		ctx.err = errors.New("can not find action:" + ctx.ActionInfo.Name)
		return
	}

	ihandler := act.instancesPool.Get()
	if ihandler == nil {
		ctx.err = errors.New("can not get action handle from pool:" + ctx.ActionInfo.Name)
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
				structType := procOutputType.Type.Field(i)
				structValue := procOutputVal.Field(i)
				structKey := structType.Tag.Get("key")
				if structKey == "" {
					structKey = structType.Name
				}
				ctx.Response.Output.Set(structKey, structValue.Interface())
			}
		}
	}
}
