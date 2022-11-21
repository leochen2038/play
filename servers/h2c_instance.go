package servers

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type h2cInstance struct {
	info   play.InstanceInfo
	hook   play.IServerHook
	ctrl   *play.InstanceCtrl
	packer play.IPacker

	tlsConfig   *tls.Config
	httpServer  http.Server
	http2server http2.Server
}

func NewH2cInstance(name string, addr string, hook play.IServerHook, packer play.IPacker) *h2cInstance {
	if packer == nil {
		packer = packers.NewHttpPackert()
	}
	if hook == nil {
		hook = defaultHook{}
	}
	return &h2cInstance{info: play.InstanceInfo{Name: name, Address: addr, Type: play.SERVER_TYPE_H2C}, packer: packer,
		hook: hook, ctrl: new(play.InstanceCtrl)}
}

func (i *h2cInstance) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var request *play.Request
	var sess = play.NewSession(r.Context(), i)
	sess.Conn.Http.Request, sess.Conn.Http.ResponseWriter = r, w

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

func (i *h2cInstance) update(r *http.Request) error {
	if r.ProtoMajor != 2 {
		return errors.New("error proto type")
	}
	if i.httpServer.Handler == nil {
		i.http2server.IdleTimeout = 30 * time.Second
		i.httpServer.Handler = h2c.NewHandler(i, &i.http2server)
	}
	return nil
}

func (i *h2cInstance) Info() play.InstanceInfo {
	return i.info
}

func (i *h2cInstance) Ctrl() *play.InstanceCtrl {
	return i.ctrl
}

func (i *h2cInstance) Hook() play.IServerHook {
	return i.hook
}

func (i *h2cInstance) Packer() play.IPacker {
	return i.packer
}

func (i *h2cInstance) Transport(conn *play.Conn, data []byte) error {
	_, err := conn.Http.ResponseWriter.Write(data)
	conn.Http.ResponseWriter.(http.Flusher).Flush()
	return err
}

func (i *h2cInstance) Run(listener net.Listener, udplistener net.PacketConn) error {
	i.http2server.IdleTimeout = 30 * time.Second
	i.httpServer.Handler = h2c.NewHandler(i, &i.http2server)
	if i.tlsConfig != nil {
		listener = tls.NewListener(listener, i.tlsConfig)
	}
	return i.httpServer.Serve(listener)
}

func (i *h2cInstance) Close() {
	i.ctrl.WaitTask()
}

func (i *h2cInstance) WithCertificate(cert tls.Certificate) *h2cInstance {
	if i.tlsConfig == nil {
		i.tlsConfig = &tls.Config{}
	}
	i.tlsConfig.Certificates = []tls.Certificate{cert}
	i.tlsConfig.Rand = rand.Reader
	return i
}

func (i *h2cInstance) Network() string {
	return "tcp"
}
