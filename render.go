package play

import (
	"errors"
	"fmt"
	"runtime/debug"
)

var renders = make(map[string]func(Output) ([]byte, error))

func RegisterRender(name string, f func(output Output)([]byte, error)) {
	renders[name] = f
}

func RunRender(name string, output Output) (data []byte, err error) {
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
	}()
	if f, ok := renders[name]; ok {
		 return f(output)
	}

	return nil, errors.New("unable find render:"+name)
}