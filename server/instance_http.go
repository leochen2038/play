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
	onRenderHandler  func(ctx *play.Context) error
	wg               sync.WaitGroup
	httpServer       http.Server
	inputMaxSize     int64
	requestTimeout   time.Duration
}

func (i *httpInstance)SetWebsocket(websocket *websocketInstance) {
	i.websocket = websocket
}

func (i *httpInstance)SetPackerDelegate(delegate play.Packer) {
	if delegate != nil {
		i.packerDelegate = delegate
	}
}

func (i *httpInstance)RequestTimeout() time.Duration {
	return i.requestTimeout
}


func (i *httpInstance)SetOnRequestHandle(onRequest func(ctx *play.Context) error) {
	i.onRequestHandler = onRequest
}
func (i *httpInstance)SetRequestTimeOut(requestTimeout time.Duration) {
	i.requestTimeout = requestTimeout
}
func (i *httpInstance)SetFormDataSize(size int64) {
	i.inputMaxSize = size
}

func (i *httpInstance)OnRequest(ctx *play.Context) error {
	if i.onRequestHandler != nil {
		return i.onRequestHandler(ctx)
	}
	return nil
}

func (i *httpInstance)OnResponse(ctx *play.Context) error {
	if i.onRenderHandler != nil {
		return i.onRenderHandler(ctx)
	}
	return nil
}

func (i *httpInstance) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var c = new(play.Client)
	var request *play.Request
	c.Http.Request, c.Http.Response = r, w

	if i.websocket != nil {
		if conn, _ := i.websocket.update(w, r); conn != nil {
			c.Websocket.WebsocketConn = conn
			i.websocket.accept(c)
			return
		}
	}

	request, _, _ = i.packerDelegate.Read(c, nil)

	c.Http.Render = request.Render
	c.Http.Template = request.ActionName

	i.wg.Add(1)
	doRequest(i, c, request)
	i.wg.Done()
}

func NewHttpInstance(name string, addr string) *httpInstance {
	i := &httpInstance{name:name, addr:addr, packerDelegate: &packers.HttpPacker{InputMaxSize: 1024*512, DefaultRender: "json"}}
	return i
}

func (i *httpInstance)WithCertificate(cert tls.Certificate) *httpInstance {
	if i.tlsConfig == nil {
		i.tlsConfig = &tls.Config{}
	}
	i.tlsConfig.Certificates = []tls.Certificate{cert}
	i.tlsConfig.Rand = rand.Reader
	return i
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