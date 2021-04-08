package packers

import (
	"github.com/gorilla/websocket"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/middleware/golang/json"
	"github.com/leochen2038/play/parsers"
)

type WebsocketJsonPacker struct  {

}

func (m *WebsocketJsonPacker)Read(c *play.Client, data []byte) (*play.Request, []byte, error) {
	var request play.Request
	request.Respond = true
	request.ActionName, request.Render = ParseHttpPath(c.Http.Request.URL.Path)
	request.Parser = parsers.NewJsonParser(data)

	return &request, nil, nil
}

func (m *WebsocketJsonPacker)Write(c *play.Client, output play.Output) error {
	var err error
	var data []byte
	var messageType = c.Websocket.MessageType
	if messageType == 0 {
		messageType = websocket.TextMessage
	}

	if data, err = json.Marshal(output.All()); err != nil {
		return err
	}

	if err := c.Websocket.WebsocketConn.WriteMessage(messageType, data); err != nil {
		return err
	}

	return  nil
}

