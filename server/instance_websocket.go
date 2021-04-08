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
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	return true
}}

type websocketInstance struct {
	addr string
	name string
	wg           sync.WaitGroup

	tlsConfig 		 *tls.Config
	httpServer       http.Server
	packerDelegate   play.Packer

	onRequestHandler func(ctx *play.Context) error
	renderHandler  func(ctx *play.Context)
}

func (i *websocketInstance)ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var conn *websocket.Conn
	var c = new(play.Client)
	var s = play.NewSession(c, i.packerDelegate)

	if conn, err = i.update(w, r); err != nil {
		log.Fatal(err)
		return
	}

	c.Websocket.WebsocketConn = conn
	c.Http.Request, c.Http.Response = r, w

	i.accept(s)
}

func (i *websocketInstance)accept(s *play.Session) {
	if request, _, _ := i.packerDelegate.Read(s.Client, nil); request != nil {
		i.wg.Add(1)
		doRequest(i, s, request)
		i.wg.Done()
	}

	i.OnReady(s)
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

func (i *websocketInstance)OnReady(session *play.Session) {
	for {
		messageType, message, err := session.Client.Websocket.WebsocketConn.ReadMessage()
		if err != nil {
			if err == io.EOF {
				log.Println("close")
			}
			log.Println("[websocket server] websocket:", err, "failure")
			return
		}

		session.Client.Websocket.MessageType = messageType
		request, _, err := i.packerDelegate.Read(session.Client, message)
		if request != nil {
			i.wg.Add(1)
			doRequest(i, session, request)
			i.wg.Done()
		}
	}
}

func NewWebsocketInstance(name string, addr string, packer play.Packer, render func(ctx *play.Context)) *websocketInstance {
	i := &websocketInstance{name: name, addr:addr}
	if packer != nil {
		i.packerDelegate = packer
	} else {
		i.packerDelegate = new(packers.WebsocketJsonPacker)
	}
	if render != nil {
		i.renderHandler = render
	} else {
		i.renderHandler = func(ctx *play.Context) {
			_ = ctx.Session.Write(ctx.Output)
		}
	}
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

func (i *websocketInstance)SetPackerDelegate(delegate play.Packer) {
	if delegate != nil {
		i.packerDelegate = delegate
	}
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

func (i *websocketInstance)SetRenderHandler(handler func(ctx *play.Context)) {
	i.renderHandler = handler
}

func (i *websocketInstance)Render(ctx *play.Context) {
	if i.renderHandler != nil {
		i.renderHandler(ctx)
	}
}

func (i *websocketInstance)SetOnRequestHandler(handler func(ctx *play.Context) error) {
	i.onRequestHandler = handler
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