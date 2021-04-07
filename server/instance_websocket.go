package server

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	return true
}}

type websocketInstance struct {
	addr string
	name string
	defaultRender string
	requestTimeout time.Duration
	wg           sync.WaitGroup

	tlsConfig 		 *tls.Config
	httpServer       http.Server
	packerDelegate   play.Packer
	inputMaxSize     int64
	onAcceptHandler  func(client *play.Client) (*play.Request, error)
	onRequestHandler func(ctx *play.Context) error
	onRenderHandler  func(ctx *play.Context) error
}

func (i *websocketInstance)ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var conn *websocket.Conn
	var client = new(play.Client)

	if conn, err = i.update(w, r); err != nil {
		log.Fatal(err)
		return
	}

	client.Websocket.WebsocketConn = conn
	client.Http.Request, client.Http.Response = r, w
	i.accept(client)
}

func (i *websocketInstance)accept(c *play.Client) {
	var err error
	var request *play.Request
	if i.onAcceptHandler != nil {
		if request, err = i.onAcceptHandler(c); err != nil {
			log.Println("[websocket server] upgrade websocket:", err, "failure")
			return
		}
	}
	if request == nil {
		request = new(play.Request)
		request.ActionName, request.Render = packers.ParseHttpPath(c.Http.Request.URL.Path)
		request.Parser = packers.ParseHttpInput(c.Http.Request, i.inputMaxSize)
	}

	i.wg.Add(1)
	doRequest(i, c, request)
	i.wg.Done()

	i.OnReady(c)
}

func (i *websocketInstance)update(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	if len(r.Header["Upgrade"]) == 0 {
		return nil, errors.New("err websocket connect")
	}

	if r.Header["Upgrade"][0] != "websocket" {
		return nil, errors.New("err websocket connect")
	}
	if conn, err := upgrader.Upgrade(w, r, nil); err != nil {
		return nil, errors.New("[websocket server] upgrade websocket failure:" +  err.Error())
	} else {
		return conn, nil
	}
}

func (i *websocketInstance)OnReady(c *play.Client) {
	for {
		messageType, message, err := c.Websocket.WebsocketConn.ReadMessage()
		if err != nil {
			if err == io.EOF {
				log.Println("close")
			}
			log.Println("[websocket server] websocket:", err, "failure")
			return
		}

		c.Websocket.MessageType = messageType
		request, _, err := i.packerDelegate.Read(c, message)
		if request != nil {
			i.wg.Add(1)
			doRequest(i, c, request)
			i.wg.Done()
		}
	}
}

func NewWebsocketInstance(name string, addr string) *websocketInstance {
	i := &websocketInstance{name: name, addr:addr, defaultRender: "json"}
	i.packerDelegate = new(packers.WebsocketJsonPacker)
	return i
}

func (i *websocketInstance)WithCertificate(cert tls.Certificate) *websocketInstance {
	if i.tlsConfig == nil {
		i.tlsConfig = &tls.Config{}
	}
	i.tlsConfig.Certificates = []tls.Certificate{cert}
	i.tlsConfig.Rand = rand.Reader
	return i
}

func (i *websocketInstance)SetOnAcceptHandler(handler func(client *play.Client) (*play.Request, error)) {
	i.onAcceptHandler = handler
}

func (i *websocketInstance)InputMaxSize() int64 {
	return i.inputMaxSize
}

func (i *websocketInstance)DefaultRender() string {
	return i.defaultRender
}

func (i *websocketInstance)SetPackerDelegate(delegate play.Packer) {
	if delegate != nil {
		i.packerDelegate = delegate
	}
}


func (i *websocketInstance)RequestTimeout() time.Duration {
	return i.requestTimeout
}

func (i *websocketInstance)Address() string {
	return i.addr
}
func (i *websocketInstance)Name() string {
	return i.name
}
func (i *websocketInstance)Type() int {
	return TypeWebsocket
}

func (i *websocketInstance)OnRequest(ctx *play.Context) error {
	if i.onRequestHandler != nil {
		return i.onRequestHandler(ctx)
	}
	return nil
}
func (i *websocketInstance)OnResponse(ctx *play.Context) error {
	if i.onRenderHandler != nil {
		return i.onRenderHandler(ctx)
	}
	return nil
}

func (i *websocketInstance)Run(listener net.Listener) error {
	i.httpServer.Handler = i
	if i.tlsConfig != nil {
		listener = tls.NewListener(listener, i.tlsConfig)
	}
	var err = i.httpServer.Serve(listener)
	return err
}

func (i *websocketInstance)Close() {
	i.wg.Wait()
}

func (i *websocketInstance)Packer() play.Packer {
	return i.packerDelegate
}