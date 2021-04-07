package play

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/leochen2038/play/parsers"
	"net"
	"runtime"
	"strconv"
	"time"
)

var (
	intranetIp      net.IP = nil
)

//type TcpPacker interface {
//	Unpack([]byte) (*Protocol, []byte, error)
//	Pack([]byte) []byte
//}

//type Request struct {
//	InstanceType int
//	Version  byte
//	Format string
//	Render string
//	Caller string
//	Tag    string
//	TraceId  string
//	SpanId   []byte
//	Respond  bool
//	ActionName   string
//	Parser parsers.Parser
//	Http *http.Request
//}

//type Request struct {
//	Protocol Protocol
//	Http *http.Request
//}


type ActionInfo struct {
	Name string
	Tag string
	RequestTime time.Time
	Timeout time.Duration
	Respond bool
}

type TraceContext struct {
	TraceId string
	SpanId byte
	StartTime time.Time
	FinishTime time.Time
	ParentSpanId []byte
	OperationName string
	ServerName string
}

type Context struct {
	ActionInfo ActionInfo
	Input       *Input
	Output      Output
	Session	*Session
	Trace	TraceContext
	Err error
	ctx		context.Context
}

func NewContextWithRequest(i ServerInstance, action ActionInfo, inputParser parsers.Parser, trace TraceContext, c *Client) *Context {
	return &Context{
		ActionInfo: action,
		Input: NewInput(inputParser),
		Output: &playKvOutput{},
		Trace: trace,
		Session: NewSession(c, i.Packer()),
		ctx:context.Background(),
	}
}

func (ctx Context)Context() context.Context {
	return ctx.ctx
}

// 根据ip，按时间生成28位Id
func Generate28Id(prefix string, suffix string, ipv4 net.IP) string {
	var x uint16
	var timeNow = time.Now()

	if ipv4 == nil {
		ipv4 = GetIntranetIp().To4()
	}
	bytesBuffer := bytes.NewBuffer(ipv4[2:])
	_ = binary.Read(bytesBuffer, binary.BigEndian, &x)
	return prefix + timeNow.Format("20060102150405") + fmt.Sprintf("%05d%04d%05d", x%0xffff, GetGoroutineID()%10000, timeNow.UnixNano()/1e3%100000) + suffix
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
