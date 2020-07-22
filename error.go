package play

import (
	"runtime"
	"strconv"
	"strings"
)

type ErrorCode struct {
	c int
	i string
	t []string
}

func (e *ErrorCode) Error() string {
	return e.i + " @" + strings.Join(e.t, " -> ")
}

func (e *ErrorCode) Code() int {
	return e.c
}

func (e *ErrorCode) Info() string {
	return e.i
}

func NewError(info string, code int, previous error) *ErrorCode {
	var track []string
	if previous != nil {
		if bar, ok := previous.(*ErrorCode); ok {
			track = bar.t
			info = info + ", " + bar.i
		} else {
			info = info + ", " + previous.Error()
		}
	}

	if track == nil {
		for i := 1; i < 16; i++ {
			if funcptr, file, line, ok := runtime.Caller(i); ok {
				if i > 1 && strings.HasSuffix(file, "action.go") {
					break
				}
				funcName := runtime.FuncForPC(funcptr).Name()
				track = append(track, file+":"+strconv.Itoa(line)+":"+funcName[strings.Index(funcName, ".")+1:]+"()")
				if strings.HasPrefix(funcName, "main.main") {
					break
				}
			} else {
				break
			}
		}
	}

	return &ErrorCode{c: code, i: info, t: track}
}
