package server

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
	"log"
	"net"
	"net/http"
	"sync"
)

type sseInstance struct {
	addr  string
	name  string
	appId int
	wg    sync.WaitGroup

	tlsConfig  *tls.Config
	httpServer http.Server

	packerDelegate   play.Packer
	onRequestHandler func(ctx *play.Context) error
	responseHandler  func(ctx *play.Context)
}

func NewSSEInstance(name string, addr string, packer play.Packer, response func(ctx *play.Context)) *sseInstance {
	i := &sseInstance{name: name, addr: addr}
	if packer != nil {
		i.packerDelegate = packer
	} else {
		i.packerDelegate = new(packers.SSEPacker)
	}
	if response != nil {
		i.responseHandler = response
	} else {
		i.responseHandler = func(ctx *play.Context) {
			_ = ctx.Session.Write(ctx.Output)
		}
	}

	return i
}

func (i *sseInstance) WithCertificate(cert tls.Certificate) *sseInstance {
	if i.tlsConfig == nil {
		i.tlsConfig = &tls.Config{}
	}
	i.tlsConfig.Certificates = []tls.Certificate{cert}
	i.tlsConfig.Rand = rand.Reader
	return i
}

func (i *sseInstance) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var c = new(play.Conn)
	var s = play.NewSession(c, i.packerDelegate)
	defer s.Close()

	c.Http.Request, c.Http.Response = r, w
	if err = i.update(w, r); err != nil {
		log.Println(err)
		return
	}
	i.accept(s)
}

func (i *sseInstance) update(w http.ResponseWriter, r *http.Request) error {
	accept := r.Header["Accept"]
	if !(len(accept) > 0 && accept[0] == "text/event-stream") {
		return errors.New("error event-stream accept type")
	}
	return nil
}

func (i *sseInstance) accept(s *play.Session) {
	var w = s.Conn.Http.Response
	_, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	request, _, err := i.packerDelegate.Read(s.Conn, nil)
	if err != nil {
		return
	}

	doRequest(i, s, request)
	<-s.Conn.Http.Request.Context().Done()
	s.Close()
}

func (i *sseInstance) SetPackerDelegate(delegate play.Packer) {
	if delegate != nil {
		i.packerDelegate = delegate
	}
}

func (i *sseInstance) SetOnRequestHandler(handler func(ctx *play.Context) error) {
	i.onRequestHandler = handler
}

func (i *sseInstance) SetResponseHandler(handler func(ctx *play.Context)) {
	i.responseHandler = handler
}

func (i *sseInstance) SetAppId(appId int) {
	i.appId = appId
}

func (i *sseInstance) Address() string {
	return i.addr
}
func (i *sseInstance) Name() string {
	return i.name
}
func (i *sseInstance) Type() int {
	return TypeSse
}
func (i *sseInstance) AppId() int {
	return i.appId
}

func (i *sseInstance) OnRequest(ctx *play.Context) error {
	if i.onRequestHandler != nil {
		return i.onRequestHandler(ctx)
	}
	return nil
}
func (i *sseInstance) Response(ctx *play.Context) {
	if i.responseHandler != nil {
		i.responseHandler(ctx)
	}
}
func (i *sseInstance) Run(listener net.Listener) error {
	i.httpServer.Handler = i
	if i.tlsConfig != nil {
		listener = tls.NewListener(listener, i.tlsConfig)
	}
	var err = i.httpServer.Serve(listener)
	return err
}
func (i *sseInstance) Close() {
	i.wg.Wait()
}

func (i *sseInstance) Packer() play.Packer {
	return i.packerDelegate
}
