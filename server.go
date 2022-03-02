package play

import (
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"reflect"
	"sync"
)

type IServerHook interface {
	OnConnect(sess *Session, err error)
	OnClose(sess *Session, err error)

	OnRequest(ctx *Context)
	OnResponse(ctx *Context)
	OnFinish(ctx *Context)
}

type IServer interface {
	Info() InstanceInfo
	Ctrl() *InstanceCtrl
	Hook() IServerHook
	Transport() ITransport

	Run(net.Listener) error
	Close()
}

type Binder interface {
	Bind(v reflect.Value) error
	Get(key string) (interface{}, error)
	Set(key string, val interface{})
}

type ITransport interface {
	Receive(c *Conn) (*Request, error)
	Response(c *Conn, res *Response) error
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
}

type Request struct {
	Version     byte
	Render      string
	Caller      string
	TagId       int
	TraceId     string
	SpanId      []byte
	Respond     bool
	ActionName  string
	InputBinder Binder
}

type Response struct {
	ErrorCode int
	TagId     int
	Render    string
	TraceId   string
	SpanId    []byte
	Template  string
	Output    Output
}
