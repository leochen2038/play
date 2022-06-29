package play

import (
	"net"
	"net/http"
	"reflect"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/leochen2038/play/codec/binder"
	"github.com/leochen2038/play/codec/render"
)

type IServerHook interface {
	OnConnect(sess *Session, err error)
	OnClose(sess *Session, err error)

	OnRequest(ctx *Context) error
	OnResponse(ctx *Context) error
	OnFinish(ctx *Context) error
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
	Send(c *Conn, res *Response) error
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
	Version      byte
	Render       string
	Caller       string
	TagId        int
	TraceId      string
	SpanId       []byte
	Respond      bool
	ActionName   string
	InputBinder  binder.Binder
	OutputRender render.Render
}

type Response struct {
	ErrorCode    int
	TagId        int
	TraceId      string
	SpanId       []byte
	Template     string
	OutputRender Output
}
