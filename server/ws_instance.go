package server

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/leochen2038/play"
	"net"
	"net/http"
	"runtime/debug"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	return true
}}

type wsInstance struct {
	info      play.InstanceInfo
	hook      play.IServerHook
	ctrl      *play.InstanceCtrl
	transport play.ITransport

	tlsConfig  *tls.Config
	httpServer http.Server
}

func NewWsInstance(name string, addr string, transport play.ITransport, hook play.IServerHook) (*wsInstance, error) {
	if transport == nil {
		return nil, errors.New("ws instance transport must not be nil")
	}
	if hook == nil {
		return nil, errors.New("ws instance hook must not be nil")
	}
	return &wsInstance{info: play.InstanceInfo{Name: name, Address: addr, Type: TypeWebsocket}, transport: transport, hook: hook, ctrl: new(play.InstanceCtrl)}, nil
}

func (i *wsInstance) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var conn *websocket.Conn
	var sess = play.NewSession(r.Context(), nil, i)

	defer func() {
		recover()
	}()

	if conn, err = i.update(w, r); err != nil {
		i.hook.OnConnect(sess, err)
		return
	}

	sess.Conn = new(play.Conn)
	sess.Conn.Websocket.WebsocketConn = conn
	sess.Conn.Http.Request, sess.Conn.Http.ResponseWriter = r, w

	i.accept(sess)
}

func (i *wsInstance) accept(s *play.Session) {
	var err error
	var request *play.Request

	i.hook.OnConnect(s, nil)
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
		i.hook.OnClose(s, err)
	}()

	if request, err = i.transport.Receive(s.Conn); request != nil {
		if err = doRequest(s, request); err != nil {
			return
		}
	}

	err = i.onReady(s)
}

func (i *wsInstance) onReady(sess *play.Session) error {
	for {
		sessContext := sess.Context()
		select {
		case <-sessContext.Done():
			return sessContext.Err()
		default:
			messageType, message, err := sess.Conn.Websocket.WebsocketConn.ReadMessage()
			if err != nil {
				return err
			}

			sess.Conn.Websocket.Message = message
			sess.Conn.Websocket.MessageType = messageType

			request, err := i.transport.Receive(sess.Conn)
			if request != nil {
				if err := doRequest(sess, request); err != nil {
					return err
				}
			}
		}
	}
}

func (i *wsInstance) update(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	if len(r.Header["Upgrade"]) == 0 {
		return nil, errors.New("err websocket connect")
	}

	if r.Header["Upgrade"][0] != "websocket" {
		return nil, errors.New("err websocket connect")
	}
	if conn, err := upgrader.Upgrade(w, r, nil); err != nil {
		return nil, errors.New("[websocket server] upgrade websocket failure:" + err.Error())
	} else {
		return conn, nil
	}
}

func (i *wsInstance) Info() play.InstanceInfo {
	return i.info
}

func (i *wsInstance) Transport() play.ITransport {
	return i.transport
}

func (i *wsInstance) Hook() play.IServerHook {
	return i.hook
}

func (i *wsInstance) Ctrl() *play.InstanceCtrl {
	return i.ctrl
}

func (i *wsInstance) WithCertificate(cert tls.Certificate) *wsInstance {
	if i.tlsConfig == nil {
		i.tlsConfig = &tls.Config{}
	}
	i.tlsConfig.Certificates = []tls.Certificate{cert}
	i.tlsConfig.Rand = rand.Reader
	return i
}

func (i *wsInstance) Run(listener net.Listener) error {
	i.httpServer.Handler = i
	if i.tlsConfig != nil {
		listener = tls.NewListener(listener, i.tlsConfig)
	}
	var err = i.httpServer.Serve(listener)
	return err
}

func (i *wsInstance) Close() {
	i.ctrl.WaitTask()
}
