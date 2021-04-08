package server

import (
	"crypto/rand"
	"crypto/tls"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
	"net"
	"net/http"
	"sync"
	"time"
)

type httpInstance struct {
	defaultRender    string
	addr             string
	name             string
	websocket        *websocketInstance

	tlsConfig 		 *tls.Config
	packerDelegate   play.Packer
	onRequestHandler func(ctx *play.Context) error
	renderHandler  func(ctx *play.Context)
	wg               sync.WaitGroup
	httpServer       http.Server
	requestTimeout   time.Duration
}

func (i *httpInstance)SetWebsocket(websocket *websocketInstance) {
	i.websocket = websocket
}

func NewHttpInstance(name string, addr string, packer play.Packer, render func(ctx *play.Context) ) *httpInstance {
	i := &httpInstance{name:name, addr:addr}
	if packer != nil {
		i.packerDelegate = packer
	} else {
		i.packerDelegate = &packers.HttpPacker{InputMaxSize:1024*4, DefaultRender:"json"}
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

func (i *httpInstance) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var request *play.Request
	var c = new(play.Client)
	var s = play.NewSession(c, i.packerDelegate)
	defer s.Close()

	c.Http.Request, c.Http.Response = r, w
	if i.websocket != nil {
		if conn, _ := i.websocket.update(w, r); conn != nil {
			c.Websocket.WebsocketConn = conn
			i.websocket.accept(s)
			return
		}
	}

	request, _, _ = i.packerDelegate.Read(c, nil)

	c.Http.Render = request.Render
	c.Http.Template = request.ActionName

	i.wg.Add(1)
	doRequest(i, s, request)
	i.wg.Done()
}



func (i *httpInstance)WithCertificate(cert tls.Certificate) *httpInstance {
	if i.tlsConfig == nil {
		i.tlsConfig = &tls.Config{}
	}
	i.tlsConfig.Certificates = []tls.Certificate{cert}
	i.tlsConfig.Rand = rand.Reader
	return i
}

func (i *httpInstance)SetPackerDelegate(delegate play.Packer) {
	if delegate != nil {
		i.packerDelegate = delegate
	}
}

func (i *httpInstance)SetOnRequestHandler(handler func(ctx *play.Context) error) {
	i.onRequestHandler = handler
}

func (i *httpInstance)OnRequest(ctx *play.Context) error {
	if i.onRequestHandler != nil {
		return i.onRequestHandler(ctx)
	}
	return nil
}

func (i *httpInstance)Render(ctx *play.Context) {
	if i.renderHandler != nil {
		i.renderHandler(ctx)
	}
}

func (i *httpInstance)SetRenderHandler(handler func (ctx *play.Context)) {
	i.renderHandler = handler
}


func (i *httpInstance)Address() string {
	return i.addr
}

func (i *httpInstance)Name() string {
	return i.name
}

func (i *httpInstance)Type() int {
	return TypeHttp
}

func (i *httpInstance)Run(listener net.Listener) error {
	i.httpServer.Handler = i
	if i.tlsConfig != nil {
		listener = tls.NewListener(listener, i.tlsConfig)
	}
	var err = i.httpServer.Serve(listener)
	return err
}

func (i *httpInstance)Close() {
	i.wg.Wait()
}

func (i *httpInstance)Packer() play.Packer {
	return i.packerDelegate
}