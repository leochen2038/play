package server

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/leochen2038/play"
	"net"
	"net/http"
	"runtime/debug"
)

type sseInstance struct {
	info      play.InstanceInfo
	hook      play.IServerHook
	ctrl      *play.InstanceCtrl
	transport play.ITransport

	tlsConfig  *tls.Config
	httpServer http.Server
}

func NewSSEInstance(name string, addr string, transport play.ITransport, hook play.IServerHook) (*sseInstance, error) {
	if transport == nil {
		return nil, errors.New("sse instance transport must not be nil")
	}
	if hook == nil {
		return nil, errors.New("sse instance server hook must not be nil")
	}
	return &sseInstance{info: play.InstanceInfo{Name: name, Address: addr, Type: TypeSse}, transport: transport, hook: hook, ctrl: new(play.InstanceCtrl)}, nil
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
	var sess = play.NewSession(r.Context(), nil, i)

	defer func() {
		recover()
	}()

	if err = i.update(r); err != nil {
		i.hook.OnConnect(sess, err)
		return
	}

	sess.Conn = new(play.Conn)
	sess.Conn.Http.Request, sess.Conn.Http.ResponseWriter = r, w
	i.accept(sess)
}

func (i *sseInstance) update(r *http.Request) error {
	accept := r.Header["Accept"]
	if !(len(accept) > 0 && accept[0] == "text/event-stream") {
		return errors.New("error event-stream accept type")
	}
	return nil
}

func (i *sseInstance) accept(s *play.Session) {
	var err error
	var w = s.Conn.Http.ResponseWriter

	i.hook.OnConnect(s, nil)
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
		i.hook.OnClose(s, err)
	}()

	if _, ok := w.(http.Flusher); !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		err = errors.New("streaming unsupported")
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Accel-Buffering", "no")

	request, err := i.transport.Receive(s.Conn)
	if err != nil {
		return
	}

	if err = doRequest(s, request); err != nil {
		return
	}

	<-s.Context().Done()
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
	i.ctrl.WaitTask()
}

func (i *sseInstance) Info() play.InstanceInfo {
	return i.info
}

func (i *sseInstance) Hook() play.IServerHook {
	return i.hook
}

func (i *sseInstance) Transport() play.ITransport {
	return i.transport
}

func (i *sseInstance) Ctrl() *play.InstanceCtrl {
	return i.ctrl
}
