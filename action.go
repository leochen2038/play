package play

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"unsafe"
)

type Processor interface {
	Run(ctx *Context) (string, error)
}

func NewProcessorWrap(handle interface{ Processor }, run func(p Processor, ctx *Context) (string, error), next map[string]*ProcessorWrap) *ProcessorWrap {
	return &ProcessorWrap{p: handle, run: run, next: next}
}

type ProcessorWrap struct {
	p    Processor
	run  func(p Processor, ctx *Context) (string, error)
	next map[string]*ProcessorWrap
}

var actionPools = make(map[string]*sync.Pool, 32)

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
		if err := ctx.Input.Bind(p); err != nil {
			return "", err
		}
	}
	return p.Run(ctx)
}

func RunAction(name string, ctx *Context) (err error) {
	pool, ok := actionPools[name]
	if !ok {
		return errors.New("can not find action:" + name)
	}

	ihandler := pool.Get()
	if ihandler == nil {
		return
	}
	defer func() {
		pool.Put(ihandler)
		if panicInfo := recover(); panicInfo != nil {
			err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
	}()

	handler := ihandler.(*ProcessorWrap)
	ctx.ActionName = name
	currentHandler := handler
	for ok := true; ok; currentHandler, ok = currentHandler.next[ctx.doneFlag] {
		if ctx.doneFlag, err = currentHandler.run(currentHandler.p, ctx); err != nil {
			return
		}
	}

	return
}
