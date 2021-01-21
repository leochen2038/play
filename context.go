package play

import (
	"bytes"
	"encoding/binary"
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
	ProtocolVersion byte   = 3
	intranetIp      net.IP = nil
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

func Generate28Id(prefix string) string {
	var x uint16
	var timeNow = time.Now()

	ipv4 := GetIntranetIp().To4()
	bytesBuffer := bytes.NewBuffer(ipv4[2:])
	_ = binary.Read(bytesBuffer, binary.BigEndian, &x)
	return prefix + timeNow.Format("20060102150405") + fmt.Sprintf("%05d%04d%05d", x%0xffff, GetGoroutineID()%10000, timeNow.UnixNano()/1e3%100000)
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

func GetMicroUqid(localaddr string) (traceId string) {
	var hexIp string
	var ip string

	if localaddr == "" {
		ipv4 := GetIntranetIp()
		for _, v := range ipv4 {
			hexIp += fmt.Sprintf("%02x", v)
		}
	} else {
		ip = localaddr[:strings.Index(localaddr, ":")]
		for j, i := 0, 0; i < len(ip); i++ {
			if ip[i] == '.' {
				hex, _ := strconv.Atoi(ip[j:i])
				hexIp += fmt.Sprintf("%02x", hex)
				j = i + 1
			}
		}
	}

	//	runtime.Gosched()
	tm := time.Now()
	micro := tm.Format(".000000")

	if len(hexIp) > 8 {
		traceId = fmt.Sprintf("%s%06s%.8s%04x", tm.Format("20060102150405"), micro[1:], hexIp, os.Getpid()%0x10000)
	} else {
		traceId = fmt.Sprintf("%s%06s%08s%04x", tm.Format("20060102150405"), micro[1:], hexIp, os.Getpid()%0x10000)
	}

	return
}

func GetGoroutineID() uint64 {
	b := make([]byte, 64)
	runtime.Stack(b, false)
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
