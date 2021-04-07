package play

import (
	"github.com/gorilla/websocket"
	"net"
	"net/http"
)

type Client struct {
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
		version int
		Conn net.Conn
	}
}