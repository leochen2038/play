package packers

import (
	"errors"
	"fmt"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/binders"
	"github.com/leochen2038/play/codec/renders"
)

const (
	renderNameJSON = "json"
)

var (
	ErrUnsupportedType = errors.New("json packer unsupported server type")
	ErrNilConnection   = errors.New("connection is nil")
	ErrNilResponse     = errors.New("response is nil")
)

type JsonPacker struct {
	httpPacker *HttpPacker
}

func NewJsonPacker() play.IPacker {
	return &JsonPacker{
		httpPacker: &HttpPacker{},
	}
}

func (p *JsonPacker) Unpack(c *play.Conn) (*play.Request, error) {
	if c == nil {
		return nil, ErrNilConnection
	}

	request := &play.Request{
		RenderName: renderNameJSON,
	}

	switch c.Type {
	case play.SERVER_TYPE_HTTP,
		play.SERVER_TYPE_SSE,
		play.SERVER_TYPE_H2C,
		play.SERVER_TYPE_HTTP3:
		return p.unpackHTTP(c, request)
	case play.SERVER_TYPE_WS:
		return p.unpackWebSocket(c, request)
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedType, c.Type)
	}
}

func (p *JsonPacker) unpackHTTP(c *play.Conn, request *play.Request) (*play.Request, error) {
	actionName, _ := p.httpPacker.ParseHttpPath(c.Http.Request.URL.Path)
	request.ActionName = actionName
	request.InputBinder = p.httpPacker.ParseHttpInput(c.Http.Request)
	return request, nil
}

func (p *JsonPacker) unpackWebSocket(c *play.Conn, request *play.Request) (*play.Request, error) {
	actionName, _ := p.httpPacker.ParseHttpPath(c.Http.Request.URL.Path)
	request.ActionName = actionName

	if len(c.Websocket.Message) > 0 {
		request.InputBinder = binders.GetBinderOfJson(c.Websocket.Message)
	} else {
		request.InputBinder = p.httpPacker.ParseHttpInput(c.Http.Request)
	}
	return request, nil
}

func (p *JsonPacker) Pack(c *play.Conn, res *play.Response) ([]byte, error) {
	if res == nil {
		return nil, ErrNilResponse
	}
	return renders.GetRenderOfJson().Render(res.Output.All())
}
