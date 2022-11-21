package servers

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
)

type httpInstance struct {
	info   play.InstanceInfo
	hook   play.IServerHook
	ctrl   *play.InstanceCtrl
	packer play.IPacker

	tlsConfig  *tls.Config
	httpServer http.Server
	ws         *wsInstance
	sse        *sseInstance
	h2c        *h2cInstance
}

func NewHttpInstance(name string, addr string, hook play.IServerHook, packer play.IPacker) *httpInstance {
	if packer == nil {
		packer = packers.NewHttpPackert()
	}
	if hook == nil {
		hook = defaultHook{}
	}
	return &httpInstance{info: play.InstanceInfo{Name: name, Address: addr, Type: play.SERVER_TYPE_HTTP}, packer: packer, hook: hook, ctrl: new(play.InstanceCtrl)}
}

func (i *httpInstance) Run(listener net.Listener, udplistener net.PacketConn) error {
	i.httpServer.Handler = i
	if i.tlsConfig != nil {
		listener = tls.NewListener(listener, i.tlsConfig)
	}
	var err = i.httpServer.Serve(listener)
	return err
}

func (i *httpInstance) Close() {
	i.ctrl.WaitTask()
}

func (i *httpInstance) Info() play.InstanceInfo {
	return i.info
}

func (i *httpInstance) Hook() play.IServerHook {
	return i.hook
}

func (i *httpInstance) Packer() play.IPacker {
	return i.packer
}

func (i *httpInstance) Transport(conn *play.Conn, data []byte) error {
	_, err := conn.Http.ResponseWriter.Write(data)
	return err
}

func (i *httpInstance) Ctrl() *play.InstanceCtrl {
	return i.ctrl
}

func (i *httpInstance) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var request *play.Request
	var sess = play.NewSession(r.Context(), i)
	sess.Conn.Http.Request, sess.Conn.Http.ResponseWriter = r, w
	if i.ws != nil {
		if conn, _ := i.ws.update(w, r); conn != nil {
			sess.Server = i.ws
			sess.Conn.Websocket.WebsocketConn = conn
			i.ws.accept(sess)
			return
		}
	}

	if i.sse != nil {
		if err = i.sse.update(r); err == nil {
			sess.Server = i.sse
			i.sse.accept(sess)
			return
		}
	}

	if i.h2c != nil {
		if err = i.h2c.update(r); err == nil && i.tlsConfig == nil {
			sess.Server = i.h2c
			i.h2c.httpServer.Handler.ServeHTTP(w, r)
			return
		}
	}

	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			fmt.Printf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
	}()
	defer func() {
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
		i.hook.OnClose(sess, err)
	}()
	i.hook.OnConnect(sess, nil)
	if request, err = i.packer.Receive(sess.Conn); err != nil {
		return
	}
	err = doRequest(r.Context(), sess, request)
}

func (i *httpInstance) SetWSInstance(ws *wsInstance) {
	i.ws = ws
}

func (i *httpInstance) SetSSEInstance(sse *sseInstance) {
	i.sse = sse
}

func (i *httpInstance) SetH2cInstance(h2c *h2cInstance) {
	i.h2c = h2c
}

func (i *httpInstance) WithCertificate(cert tls.Certificate) *httpInstance {
	if i.tlsConfig == nil {
		i.tlsConfig = &tls.Config{}
	}
	i.tlsConfig.NextProtos = []string{"h2"}
	i.tlsConfig.Certificates = []tls.Certificate{cert}
	i.tlsConfig.Rand = rand.Reader
	return i
}

func (i *httpInstance) Network() string {
	return "tcp"
}
