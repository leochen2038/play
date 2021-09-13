package transport

import (
	"github.com/gorilla/websocket"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/binder"
	"github.com/leochen2038/play/library/golang/json"
)

type WebsocketJsonPacker struct {
}

func (m *WebsocketJsonPacker) Receive(c *play.Conn) (*play.Request, error) {
	var request play.Request
	request.Respond = true
	request.ActionName, request.Render = ParseHttpPath(c.Http.Request.URL.Path)
	request.InputBinder = binder.NewJsonBinder(c.Websocket.Message)

	return &request, nil
}

func (m *WebsocketJsonPacker) Send(c *play.Conn, res *play.Response) error {
	var err error
	var data []byte
	var messageType = c.Websocket.MessageType

	if messageType == 0 {
		messageType = websocket.TextMessage
	}

	if data, err = json.Marshal(res.Output.All()); err != nil {
		return err
	}

	if err := c.Websocket.WebsocketConn.WriteMessage(messageType, data); err != nil {
		return err
	}

	return nil
}