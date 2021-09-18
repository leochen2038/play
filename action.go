package play

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"unsafe"
)

var actionPools = make(map[string]*sync.Pool, 32)

type Processor interface {
	Run(ctx *Context) (string, error)
}

func GetActionPools() map[string]*sync.Pool {
	return actionPools
}

func NewProcessorWrap(handle interface{ Processor }, run func(p Processor, ctx *Context) (string, error), next map[string]*ProcessorWrap) *ProcessorWrap {
	return &ProcessorWrap{p: handle, run: run, next: next}
}

type ProcessorWrap struct {
	p    Processor
	run  func(p Processor, ctx *Context) (string, error)
	next map[string]*ProcessorWrap
}

func RegisterAction(name string, new func() interface{}) {
	actionPools[name] = &sync.Pool{New: new}
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

func RunAction(ctx *Context) (err error) {
	var flag string
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
	}()
	pool, ok := actionPools[ctx.ActionInfo.Name]
	if !ok {
		return errors.New("can not find action:" + ctx.ActionInfo.Name)
	}

	ihandler := pool.Get()
	if ihandler == nil {
		return errors.New("can not get action handle from pool:" + ctx.ActionInfo.Name)
	}
	defer pool.Put(ihandler)

	// set context
	if ctx.ActionInfo.Timeout > 0 {
		var cancel context.CancelFunc
		ctx.ctx, cancel = context.WithTimeout(ctx.ctx, ctx.ActionInfo.Timeout)
		defer cancel()
	}

	currentHandler := ihandler.(*ProcessorWrap)
	for ok := true; ok; currentHandler, ok = currentHandler.next[flag] {
		flag, err = currentHandler.run(currentHandler.p, ctx)
		if ctx.ctx.Err() != nil {
			if err != nil {
				return err
			}
			return ctx.ctx.Err()
		}
		if err != nil {
			return err
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
	return
}
