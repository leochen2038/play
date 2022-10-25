package packers

import (
	"errors"
	"strconv"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/binders"
	"github.com/leochen2038/play/codec/renders"
)

type JsonPacker struct {
}

func NewJsonPackert() play.IPacker {
	return new(JsonPacker)
}

func (m *JsonPacker) Receive(c *play.Conn) (*play.Request, error) {
	var request play.Request
	request.RenderName = "json"

	switch c.Type {
	case play.SERVER_TYPE_HTTP, play.SERVER_TYPE_SSE, play.SERVER_TYPE_H2C, play.SERVER_TYPE_HTTP3:
		request.ActionName, _ = ParseHttpPath(c.Http.Request.URL.Path)
		request.InputBinder = ParseHttpInput(c.Http.Request)
	case play.SERVER_TYPE_WS:
		request.ActionName, _ = ParseHttpPath(c.Http.Request.URL.Path)
		request.InputBinder = binders.GetBinderOfJson(c.Websocket.Message)
	default:
		return nil, errors.New("json packer not support " + strconv.Itoa(c.Type) + " type")
	}

	return &request, nil
}

func (m *JsonPacker) Pack(c *play.Conn, res *play.Response) (data []byte, err error) {
	return renders.GetRenderOfJson().Render(res.Output.All())
}
