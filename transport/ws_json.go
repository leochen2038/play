package transport

import (
	"github.com/gorilla/websocket"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/binder"
	"github.com/leochen2038/play/library/golang/json"
)

type WsJsonTransport struct {
}

func NewWsJsonTransport() *WsJsonTransport {
	return new(WsJsonTransport)
}

func (m *WsJsonTransport) Receive(c *play.Conn) (*play.Request, error) {
	var request play.Request
	request.Respond = true
	request.ActionName, request.Render = ParseHttpPath(c.Http.Request.URL.Path)

	if len(c.Websocket.Message) > 0 {
		request.InputBinder = binder.NewJsonBinder(c.Websocket.Message)
	} else {
		request.InputBinder = ParseHttpInput(c.Http.Request, 4096)
	}

	return &request, nil
}

func (m *WsJsonTransport) Response(c *play.Conn, res *play.Response) error {
	var err error
	var data []byte
	var messageType = c.Websocket.MessageType

	if messageType == 0 {
		messageType = websocket.TextMessage
	}

	if data, err = json.MarshalEscape(res.Output.All(), false, false); err != nil {
		return err
	}

	if err := c.Websocket.WebsocketConn.WriteMessage(messageType, data); err != nil {
		return err
	}

	return nil
}
