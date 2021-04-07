package server

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type sseInstance struct {
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
	onResponseHandler  func(ctx *play.Context) error
}

func NewSSEInstance(name string, addr string) *sseInstance {
	i := &sseInstance{name: name, addr:addr, defaultRender: "json"}
	i.packerDelegate = new(packers.SSEPacker)
	return i
}

func (i *sseInstance)WithCertificate(cert tls.Certificate) *sseInstance {
	if i.tlsConfig == nil {
		i.tlsConfig = &tls.Config{}
	}
	i.tlsConfig.Certificates = []tls.Certificate{cert}
	i.tlsConfig.Rand = rand.Reader
	return i
}


func (i *sseInstance)ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var client = new(play.Client)
	client.Http.Request, client.Http.Response = r, w

	if err = i.update(w, r); err != nil {
		log.Println(err)
		return
	}
	i.accept(client)
}

func (i *sseInstance)update(w http.ResponseWriter, r *http.Request) error {
	accept := r.Header["Accept"]
	if !(len(accept) > 0 && accept[0] == "text/event-stream") {
		return errors.New("error event-stream accept type")
	}
	return nil
}

func (i *sseInstance)accept(c *play.Client) {
	var w = c.Http.Response
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

	request, _, err := i.packerDelegate.Read(c, nil)
	if err != nil {
		return
	}

	doRequest(i, c, request)
	<-c.Http.Request.Context().Done()
	fmt.Println("close sse")
}


func (i *sseInstance)InputMaxSize() int64 {
	return i.inputMaxSize
}

func (i *sseInstance)DefaultRender() string {
	return i.defaultRender
}

func (i *sseInstance)SetPackerDelegate(delegate play.Packer) {
	if delegate != nil {
		i.packerDelegate = delegate
	}
}
func (i *sseInstance)SetOnAcceptHandler(handler func(client *play.Client) (*play.Request, error)) {
	i.onAcceptHandler = handler
}

func (i *sseInstance)RequestTimeout() time.Duration {
	return i.requestTimeout
}

func (i *sseInstance)Address() string {
	return i.addr
}
func (i *sseInstance)Name() string {
	return i.name
}
func (i *sseInstance)Type() int {
	return TypeSse
}

func (i *sseInstance)OnRequest(ctx *play.Context) error {
	if i.onRequestHandler != nil {
		return i.onRequestHandler(ctx)
	}
	return nil
}
func (i *sseInstance)OnResponse(ctx *play.Context) error {
	if i.onResponseHandler != nil {
		return i.onResponseHandler(ctx)
	}
	return nil
}
func (i *sseInstance)Run(listener net.Listener) error {
	i.httpServer.Handler = i
	if i.tlsConfig != nil {
		listener = tls.NewListener(listener, i.tlsConfig)
	}
	var err = i.httpServer.Serve(listener)
	return err
}
func (i *sseInstance)Close() {
	i.wg.Wait()
}

func (i *sseInstance)Packer() play.Packer {
	return i.packerDelegate
}