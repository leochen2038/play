package logger

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/leochen2038/play/codec/protos/golang/json"
)

const LEVEL_DEBUG = 3
const LEVEL_INFO = 2
const LEVEL_WARN = 1
const LEVEL_ERROR = 0

var level = 3
var wchan chan *log
var lvMap = map[int]string{3: "debug", 2: "info", 1: "warn", 0: "error"}

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
	wchan = make(chan *log, 64)
	go writerWith2Day(wchan)
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

func Write(lv int, now time.Time, traceId string, action string, file string, k string, v interface{}, attach map[string]interface{}) {
	if level < lv {
		return
	}
	data := fmt.Sprintf(`{"time":"%s", "level":"%s", "traceId":"%s", "action":"%s", "file":"%s"`, now.Format("2006-01-02 15:04:05.000"), lvMap[lv], traceId, action, file)

	if k != "" {
		if vv, _ := json.MarshalEscape(v, false, false); vv != nil {
			data += fmt.Sprintf(`, "%s":%s`, k, vv)
		}
	}
	for k, v := range attach {
		if vv, _ := json.MarshalEscape(v, false, false); vv != nil {
			data += fmt.Sprintf(`, "%s":%s`, k, vv)
		}
	}
	data += "}\n"

	select {
	case wchan <- &log{now, level, []byte(data)}:
		return
	default:
		fmt.Println("log channel is full")
	}
}

func Info(k string, v interface{}, kv ...interface{}) {
	if level >= LEVEL_INFO {
		Write(LEVEL_INFO, time.Now(), "", "", getFile(), k, v, getAttach(kv))
	}
}

func Error(err error, kv ...interface{}) {
	Write(LEVEL_ERROR, time.Now(), "", "", getFile(), "error", err.Error(), getAttach(kv))
}

func Debug(k string, v interface{}, kv ...interface{}) {
	if level >= LEVEL_DEBUG {
		Write(LEVEL_DEBUG, time.Now(), "", "", getFile(), k, v, getAttach(kv))
	}
}

func Warn(k string, v interface{}, kv ...interface{}) {
	if level >= LEVEL_WARN {
		Write(LEVEL_WARN, time.Now(), "", "", getFile(), k, v, getAttach(kv))
	}
}

func getAttach(kv []interface{}) map[string]interface{} {
	attach := make(map[string]interface{})
	len := len(kv) - 1

	for i := 0; i < len; i += 2 {
		if v, ok := kv[i].(string); ok {
			attach[v] = kv[i+1]
		}
	}
	return attach
}

func getFile() string {
	funcptr, file, line, _ := runtime.Caller(2)
	funcName := runtime.FuncForPC(funcptr).Name()
	return fmt.Sprintf(`%s:%d->%s()`, file, line, funcName[strings.Index(funcName, ".")+1:])
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

	// 只保留最新三份日志
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

		for i := 3; i < len(logs); i++ {
			if f, ok := logfile.LoadAndDelete(logs[i].Name()); ok {
				f.(*os.File).Close()
			}
			os.Remove(path.Join(path.Dir(exeName), logs[i].Name()))
		}
	}

	return actual.(*os.File)
}
