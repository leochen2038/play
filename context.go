package play

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	BuildBasePath string
	intranetIp    net.IP = nil
	tracPrefix           = "trac-"
)

func init() {
	ip := strings.Split(GetIntranetIp().String(), ".")
	i, _ := strconv.Atoi(ip[len(ip)-1])
	tracPrefix = fmt.Sprintf("trac-%03d-", i)
}

func NewTraceId() string {
	return Generate23Id(tracPrefix, "")
}

type actionRequest struct {
	CallerId    int
	Name        string
	ActionExist bool
	RequestTime time.Time
	Timeout     time.Duration
	NonRespond  bool
}

type TraceContext struct {
	TraceId       string
	SpanId        byte
	TagId         int
	StartTime     time.Time
	FinishTime    time.Time
	ParentSpanId  []byte
	OperationName string
	ServerName    string
}

type Context struct {
	context.Context
	ServerName    string
	ActionRequest actionRequest
	Input         Input
	Response      Response
	Session       *Session
	Trace         *TraceContext
	FinishTime    time.Time
	isFinish      bool
	err           error
	gctx          context.Context
	gcfunc        context.CancelFunc
}

func NewPlayContext(parent context.Context, s *Session, request *Request, timeout time.Duration) *Context {
	var traceId string
	if request.TraceId != "" {
		traceId = request.TraceId
	} else {
		traceId = NewTraceId()
	}
	if !request.Deadline.IsZero() {
		if t := time.Until(request.Deadline); t < timeout {
			timeout = t
		}
	}
	gctx, gcfunc := context.WithTimeout(parent, timeout)
	var action = actionRequest{
		CallerId:    request.CallerId,
		Name:        request.ActionName,
		NonRespond:  request.NonRespond,
		RequestTime: time.Now(),
	}
	var trace = TraceContext{
		TagId:        request.TagId,
		TraceId:      traceId,
		ParentSpanId: request.SpanId,
		StartTime:    time.Now(),
		ServerName:   request.ActionName,
	}
	var response = Response{
		Version:    request.Version,
		TraceId:    traceId,
		RenderName: request.RenderName,
		Template:   strings.ReplaceAll(request.ActionName, ".", "/"),
	}

	return &Context{
		ActionRequest: action,
		Input:         NewInput(request.InputBinder),
		Response:      response,
		Trace:         &trace,
		Session:       s,
		gctx:          gctx,
		gcfunc:        gcfunc,
	}
}

func (c *Context) Done() <-chan struct{} {
	return c.gctx.Done()
}

func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.gctx.Deadline()
}

func (c *Context) Finish() {
	if !c.isFinish {
		c.isFinish = true
		c.FinishTime = time.Now()
		if c.err == nil {
			c.err = c.gctx.Err()
		}
		c.gcfunc()
	}
}

func (c *Context) Err() error {
	if c.err != nil {
		return c.err
	}
	if !c.isFinish {
		return c.gctx.Err()
	}
	return nil
}

func (c *Context) Value(key interface{}) interface{} {
	return c.gctx.Value(key)
}

// Generate28Id 根据ip，按时间生成28位Id
func Generate28Id(prefix string, suffix string) string {
	var x uint16
	var timeNow = time.Now()
	var ipv4 = GetIntranetIp().To4()

	bytesBuffer := bytes.NewBuffer(ipv4[2:])
	_ = binary.Read(bytesBuffer, binary.BigEndian, &x)
	return prefix + timeNow.Format("20060102150405") + fmt.Sprintf("%05d%04d%05d", x%0xffff, GetGoroutineID()%10000, timeNow.UnixNano()/1e3%100000) + suffix
}

// Generate23Id，按时间生成23位Id
func Generate23Id(prefix, suffix string) string {
	var timeNow = time.Now()
	return prefix + timeNow.Format("20060102150405") + fmt.Sprintf("%04d%05d", GetGoroutineID()%10000, timeNow.UnixNano()/1e3%100000) + suffix
}

func GetIntranetIp() net.IP {
	if intranetIp == nil {
		var err error
		var addr []net.Addr

		if addr, err = net.InterfaceAddrs(); err != nil {
			intranetIp = net.IPv4(127, 0, 0, 1)
			goto DONE
		}

		for _, value := range addr {
			if inet, ok := value.(*net.IPNet); ok && !inet.IP.IsLoopback() {
				if ipv4 := inet.IP.To4(); ipv4 != nil {
					if ipv4[0] == 10 || (ipv4[0] == 192 && ipv4[1] == 168) || (ipv4[0] == 172 && ipv4[1] >= 16 && ipv4[1] <= 31) {
						intranetIp = inet.IP
						break
					}
				}
			}
		}
	}
DONE:
	return intranetIp
}

func GetGoroutineID() uint64 {
	b := make([]byte, 64)
	runtime.Stack(b, false)
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
