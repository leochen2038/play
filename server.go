package play

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/leochen2038/play/codec/binders"
	"github.com/quic-go/quic-go"
)

const (
	SERVER_TYPE_HTTP  = 1
	SERVER_TYPE_TCP   = 2
	SERVER_TYPE_SSE   = 3
	SERVER_TYPE_WS    = 4
	SERVER_TYPE_H2C   = 5
	SERVER_TYPE_QUIC  = 6
	SERVER_TYPE_HTTP3 = 7
)

type IServerHook interface {
	OnBoot(server IServer)
	OnShutdown(server IServer)
	OnConnect(sess *Session, err error)
	OnClose(sess *Session, err error)

	OnRequest(ctx *Context) error
	OnResponse(ctx *Context)
	OnFinish(ctx *Context)
}

type IServer interface {
	Info() InstanceInfo
	Ctrl() *InstanceCtrl
	Hook() IServerHook
	Packer() IPacker
	Transport(*Conn, []byte) error
	Network() string
	Run(net.Listener, net.PacketConn) error
	Close()
}

type IPacker interface {
	Receive(c *Conn) (*Request, error)
	Pack(c *Conn, res *Response) ([]byte, error)
}

type InstanceInfo struct {
	Address string
	Name    string
	Type    int
}

type InstanceCtrl struct {
	wg sync.WaitGroup
}

func (c *InstanceCtrl) AddTask() {
	c.wg.Add(1)
}
func (c *InstanceCtrl) DoneTask() {
	c.wg.Done()
}
func (c *InstanceCtrl) WaitTask() {
	c.wg.Wait()
}

type Conn struct {
	Type    int
	IsClose bool
	Http    struct {
		Request        *http.Request
		ResponseWriter http.ResponseWriter
	}
	Websocket struct {
		Message       []byte
		MessageType   int
		WebsocketConn *websocket.Conn
	}
	Tcp struct {
		Version byte
		Surplus []byte
		Conn    net.Conn
	}
	Quic struct {
		Version byte
		Conn    quic.Connection
		Stream  quic.Stream
	}
}

type Request struct {
	Version     byte
	RenderName  string
	CallerId    int
	TagId       int
	TraceId     string
	SpanId      []byte
	NonRespond  bool
	ActionName  string
	Attach      []byte
	Deadline    time.Time
	InputBinder binders.Binder
}

type Response struct {
	Version    byte
	TraceId    string
	Template   string
	RenderName string
	Error      error
	Output     Output
}
