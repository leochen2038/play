package play

import (
	"github.com/gorilla/websocket"
	"net"
	"net/http"
)

type Conn struct {
	IsClose bool
	Http struct {
		Template string
		Render string
		Request *http.Request
		Response http.ResponseWriter
	}
	Websocket struct {
		MessageType int
		WebsocketConn *websocket.Conn
	}
	Tcp struct {
		Tag string
		TraceId string
		Version byte
		Conn net.Conn
	}
}