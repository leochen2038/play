package play

import (
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Err struct {
	id    string
	tip   string
	err   error
	time  time.Time
	kvs   []interface{}
	track []string
	code  int
}

func (e Err) Err() error {
	return e.err
}

func (e Err) AttachKv() []interface{} {
	return e.kvs
}

func (e Err) Tip() string {
	return e.tip
}

func (e Err) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return e.tip
}

func (e Err) Track() []string {
	return e.track
}

func (e Err) Time() time.Time {
	return e.time
}

func (e Err) Code() int {
	return e.code
}

func (e Err) Previous() (err Err) {
	if err, ok := e.err.(Err); ok {
		return err
	}
	return
}

func (e Err) WrapTip(tip string) Err {
	e.tip = tip
	return e
}

func (e Err) WrapCode(code int) Err {
	e.code = code
	return e
}

func (e Err) WrapKv(kv ...interface{}) Err {
	if len(kv) > 0 && len(kv)%2 == 0 {
		e.kvs = append(e.kvs, kv...)
	}
	return e
}

func WrapErr(err error, kv ...interface{}) (e Err) {
	if len(kv) > 0 && len(kv)%2 != 0 {
		kv = append(kv, "")
	}
	if e, ok := err.(Err); ok {
		e.kvs = append(e.kvs, kv...)
		return e
	}
	return _wrapErr(err, 0, "", kv)
}

func _wrapErr(e error, code int, tip string, kv []interface{}) (err Err) {
	err.err = e
	err.time = time.Now()
	err.id = Generate28Id("", "")
	err.kvs = kv
	err.code = code
	err.tip = tip

	for i := 2; i < 10; i++ {
		if funcptr, file, line, ok := runtime.Caller(i); ok {
			if i > 1 && strings.HasSuffix(file, "action.go") {
				break
			}
			funcName := runtime.FuncForPC(funcptr).Name()
			err.track = append(err.track, strings.Replace(file, BuildBasePath, "", 1)+":"+strconv.Itoa(line)+" "+funcName[strings.Index(funcName, ".")+1:]+"()")
			if strings.HasPrefix(funcName, "main.main") {
				break
			}
			continue
		}
		break
	}
	return
}
