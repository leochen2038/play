package play

import (
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Err struct {
	id     string
	tip    string
	err    error
	time   time.Time
	attach map[string]interface{}
	track  []string
	code   int
}

func (e Err) Err() error {
	return e.err
}

func (e Err) Attach() map[string]interface{} {
	return e.attach
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

func WrapErr(err error, kv ...interface{}) (e Err) {
	if e, ok := err.(Err); ok {
		len := len(kv) - 1
		for i := 0; i < len; i += 2 {
			if v, ok := kv[i].(string); ok {
				e.attach[v] = kv[i+1]
			}
		}
		return e
	}
	return _wrapErr(err, 0, "", kv)
}

func _wrapErr(e error, code int, tip string, kv []interface{}) (err Err) {
	err.err = e
	err.time = time.Now()
	err.attach = make(map[string]interface{})
	err.id = Generate28Id("", "")
	err.code = code
	err.tip = tip
	len := len(kv) - 1

	for i := 0; i < len; i += 2 {
		if v, ok := kv[i].(string); ok {
			err.attach[v] = kv[i+1]
		}
	}

	for i := 2; i < 10; i++ {
		if funcptr, file, line, ok := runtime.Caller(i); ok {
			if i > 1 && strings.HasSuffix(file, "action.go") {
				break
			}
			funcName := runtime.FuncForPC(funcptr).Name()
			err.track = append(err.track, strings.Replace(file, BuildBasePath, "", 1)+":"+strconv.Itoa(line)+"->"+funcName[strings.Index(funcName, ".")+1:]+"()")
			if strings.HasPrefix(funcName, "main.main") {
				break
			}
			continue
		}
		break
	}
	return
}
