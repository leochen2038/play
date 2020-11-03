package play

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	ProtocolVersion byte = 3
)

type Context struct {
	RequestTime time.Time
	ActionName  string
	Render      string
	Input       *Input
	Output      Output

	HttpRequest  *http.Request
	HttpResponse http.ResponseWriter
	Session      Session
	TraceId      string
	SpanId       byte
	ParentSpanId []byte
	TagId        int
	Version      byte
	doneFlag     string
}

func (ctx *Context) Done(doneFlag string) error {
	ctx.doneFlag = doneFlag
	return nil
}

func NewContextWithInput(input *Input) *Context {
	ctx := &Context{Input: input, Output: &playKvOutput{}, RequestTime: time.Now(), Version: ProtocolVersion}
	return ctx
}

func NewContextWithHttp(input *Input, r *http.Request, writer http.ResponseWriter) *Context {
	ctx := NewContextWithInput(input)
	ctx.HttpRequest = r
	ctx.HttpResponse = writer
	ctx.TraceId = GetMicroUqid("")

	return ctx
}

func NewContext(input *Input, tagId int, traceId string, parentSpanId []byte, version byte) *Context {
	ctx := NewContextWithInput(input)
	ctx.TagId = tagId
	ctx.TraceId = traceId
	ctx.ParentSpanId = parentSpanId
	ctx.Version = version

	return ctx
}

func ContextBackground() *Context {
	ctx := &Context{TraceId: GetMicroUqid(""), Version: ProtocolVersion}
	return ctx
}

func GetIntranetIp() string {
	var ip string = "127.0.0.1"
	addr, err := net.InterfaceAddrs()
	if err != nil {
		return ip
	}

	for _, value := range addr {
		if inet, ok := value.(*net.IPNet); ok && !inet.IP.IsLoopback() {
			if inet.IP.To4() != nil && strings.HasPrefix(inet.IP.String(), "192.168") {
				ip = inet.IP.String()
			}
		}
	}

	return ip
}

func GetMicroUqid(localaddr string) (traceId string) {
	var hexIp string
	var ip string

	if localaddr == "" {
		ip = GetIntranetIp()
	} else {
		ip = localaddr[:strings.Index(localaddr, ":")]
	}

	for j, i := 0, 0; i < len(ip); i++ {
		if ip[i] == '.' {
			hex, _ := strconv.Atoi(ip[j:i])
			hexIp += fmt.Sprintf("%02x", hex)
			j = i + 1
		}
	}

	runtime.Gosched()
	tm := time.Now()
	micro := tm.Format(".000000")

	if len(hexIp) > 8 {
		traceId = fmt.Sprintf("%s%06s%.8s%04x", tm.Format("20060102150405"), micro[1:], hexIp, os.Getpid()%0x10000)
	} else {
		traceId = fmt.Sprintf("%s%06s%08s%04x", tm.Format("20060102150405"), micro[1:], hexIp, os.Getpid()%0x10000)
	}

	return
}
