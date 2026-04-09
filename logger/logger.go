package logger

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/protos/golang/json"
)

const LEVEL_DEBUG = 3
const LEVEL_INFO = 2
const LEVEL_WARN = 1
const LEVEL_ERROR = 0
const LEVEL_ACCESS = -1
const LEVEL_ALERT = -2

var level = 3
var wchan chan *log
var daysToKeep = 7
var lvMap = map[int]string{3: "DEBUG", 2: "INFO", 1: "WARN", 0: "ERROR", -1: "ACCESS", -2: "ALERT"}

type log struct {
	time  time.Time
	level int
	data  []byte
}

var (
	logfile sync.Map
	exeName string
)

func init() {
	exeName, _ = os.Executable()
	wchan = make(chan *log, 256)
	go writerWith2Day(wchan)
}

func SetLogKeepDays(days int) {
	if days > 1 {
		daysToKeep = days
	} else {
		daysToKeep = 1
	}
}

func SetLevel(l int) {
	if l > 3 {
		l = 3
	}
	if l < 0 {
		l = 0
	}
	level = l
}

func Write(lv int, now time.Time, cost time.Duration, traceId string, action string, file string, message string, kv []interface{}, responseSize int) {
	if level < lv {
		return
	}
	var attach, data string
	if len(kv) > 0 && len(kv)%2 != 0 {
		kv = append(kv, "")
	}
	for i := 1; i < len(kv); i += 2 {
		if k, ok := kv[i-1].(string); ok {
			if v, _ := json.MarshalEscape(kv[i], false, false); v != nil {
				attach += fmt.Sprintf(`, "%s":%s`, k, v)
			}
		}
	}
	attach = strings.TrimLeft(attach, ", ")
	// [时间] [等级] [action] [耗时] [message] [文件路径] [自定义参数] [tracId] [responseSize]
	if responseSize > 0 {
		data = fmt.Sprintf("[%s] [%s] [%s] [%dms] [%s] [{%s}] [%s] [%s] [%d]\n", now.Format("2006-01-02 15:04:05.000"), lvMap[lv], action, int64(cost/time.Millisecond), message, file, attach, traceId, responseSize)
	} else {
		data = fmt.Sprintf("[%s] [%s] [%s] [%dms] [%s] [{%s}] [%s] [%s]\n", now.Format("2006-01-02 15:04:05.000"), lvMap[lv], action, int64(cost/time.Millisecond), message, file, attach, traceId)
	}

	select {
	case wchan <- &log{now, level, []byte(data)}:
		return
	default:
		fmt.Println("log channel is full")
	}
}

func Info(ctx context.Context, message string, kv ...interface{}) {
	var traceId, action string
	var cost time.Duration
	var now = time.Now()
	if c, ok := ctx.(*play.Context); ok {
		traceId, action = c.Trace.TraceId, c.ActionRequest.Name
		cost = now.Sub(c.ActionRequest.RequestTime)
	}
	Write(LEVEL_INFO, now, cost, traceId, action, getFile(), message, kv, 0)
}

func Alert(ctx context.Context, message string, kv ...interface{}) {
	var traceId, action string
	var cost time.Duration
	var now = time.Now()
	if c, ok := ctx.(*play.Context); ok {
		traceId, action = c.Trace.TraceId, c.ActionRequest.Name
		cost = now.Sub(c.ActionRequest.RequestTime)
	}
	Write(LEVEL_ALERT, now, cost, traceId, action, getFile(), message, kv, 0)
}

func Error(ctx context.Context, err error, kv ...interface{}) {
	var ikv []interface{}
	var traceId, action, file string
	var cost time.Duration
	var now = time.Now()
	if c, ok := ctx.(*play.Context); ok {
		traceId, action = c.Trace.TraceId, c.ActionRequest.Name
		cost = now.Sub(c.ActionRequest.RequestTime)
	}
	if playErr, ok := err.(play.Err); ok && len(playErr.Track()) > 0 {
		file = playErr.Track()[0]
		if len(playErr.AttachKv()) > 1 {
			ikv = append(ikv, playErr.AttachKv()...)
		}
		ikv = append(ikv, kv...)
	} else {
		file = getFile()
		ikv = kv
	}
	Write(LEVEL_ERROR, now, cost, traceId, action, file, err.Error(), ikv, 0)
}

func Access(ctx *play.Context) {
	var ikv []interface{}
	var file string
	now := time.Now()
	cost := now.Sub(ctx.ActionRequest.RequestTime)
	traceId, action := ctx.Trace.TraceId, ctx.ActionRequest.Name

	if ctx.Err() != nil {
		if playErr, ok := ctx.Err().(play.Err); ok && len(playErr.Track()) > 0 {
			file = playErr.Track()[0]
			ikv = append(ikv, "err", ctx.Err().Error())
			if len(playErr.AttachKv()) > 1 {
				ikv = append(ikv, playErr.AttachKv()...)
			}
			ikv = append(ikv, "input", ctx.Input.Value(""))
		} else {
			ikv = []interface{}{"err", ctx.Err().Error(), "input", ctx.Input.Value("")}
		}

		Write(LEVEL_ACCESS, now, cost, traceId, action, file, "fail", ikv, ctx.Response.ResponseSize)
	} else {
		ikv = []interface{}{"user", ctx.Session.User}
		Write(LEVEL_ACCESS, now, cost, traceId, action, file, "success", ikv, ctx.Response.ResponseSize)
	}
}

func Debug(ctx context.Context, message string, kv ...interface{}) {
	var traceId, action string
	var cost time.Duration
	var now = time.Now()
	if c, ok := ctx.(*play.Context); ok {
		traceId, action = c.Trace.TraceId, c.ActionRequest.Name
		cost = now.Sub(c.ActionRequest.RequestTime)
	}
	Write(LEVEL_DEBUG, now, cost, traceId, action, getFile(), message, kv, 0)
}

func Warn(ctx context.Context, message string, kv ...interface{}) {
	var traceId, action string
	var cost time.Duration
	var now = time.Now()
	if c, ok := ctx.(*play.Context); ok {
		traceId, action = c.Trace.TraceId, c.ActionRequest.Name
		cost = now.Sub(c.ActionRequest.RequestTime)
	}
	Write(LEVEL_WARN, now, cost, traceId, action, getFile(), message, kv, 0)
}

func getFile() string {
	funcptr, file, line, _ := runtime.Caller(2)
	funcName := runtime.FuncForPC(funcptr).Name()
	return fmt.Sprintf(`%s:%d %s()`, strings.Replace(file, play.BuildBasePath, "", 1), line, funcName[strings.Index(funcName, ".")+1:])
}

func writerWith2Day(logchan <-chan *log) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("log writer panic", err)
		}
		time.Sleep(time.Second * 3)
		go writerWith2Day(logchan)
	}()
	for log := range logchan {
		var file *os.File
		if file = fileHandler(log.time); file != nil {
			file.Write(log.data)
		}
	}
}

func fileHandler(ts time.Time) *os.File {
	var actual interface{}
	var file *os.File
	var err error
	var loaded bool
	var filename = exeName + ".log." + ts.Format("2006-01-02")

	if file, ok := logfile.Load(filename); ok {
		return file.(*os.File)
	}

	if file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666); err != nil {
		return nil
	}

	if actual, loaded = logfile.LoadOrStore(filename, file); loaded {
		file.Close()
	}

	logs := make([]os.FileInfo, 0)
	if dir, err := os.Open(path.Dir(exeName)); err == nil {
		list, _ := dir.ReadDir(0)
		for _, f := range list {
			if f.IsDir() {
				continue
			}
			if info, _ := f.Info(); info != nil {
				if strings.HasPrefix(info.Name(), fmt.Sprintf("%s.log.20", path.Base(exeName))) {
					logs = append(logs, info)
				}
			}
		}

		sort.Slice(logs, func(i, j int) bool {
			return logs[i].ModTime().After(logs[j].ModTime())
		})

		// 只保留固定天数的日志
		for i := daysToKeep; i < len(logs); i++ {
			if f, ok := logfile.LoadAndDelete(path.Join(path.Dir(exeName), logs[i].Name())); ok {
				f.(*os.File).Close()
			}
			os.Remove(path.Join(path.Dir(exeName), logs[i].Name()))
		}
	}

	return actual.(*os.File)
}
