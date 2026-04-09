package servers

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"sort"
	"time"

	"github.com/gorilla/websocket"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	return true
}}

type wsInstance struct {
	info        play.IInstanceInfo
	hook        play.IServerHook
	ctrl        *play.InstanceCtrl
	packer      play.IPacker
	actions     map[string]*play.ActionUnit
	sortedNames []string
	tlsConfig   *tls.Config
	httpServer  http.Server
}

func NewWsInstance(name string, addr string, hook play.IServerHook, packer play.IPacker, defaultActionTimeout time.Duration) *wsInstance {
	if packer == nil {
		packer = packers.NewJsonPacker()
	}
	if hook == nil {
		hook = defaultHook{}
	}
	if defaultActionTimeout == 0 {
		defaultActionTimeout = defaultTimeout
	}
	return &wsInstance{info: play.NewInstanceInfo(name, addr, play.SERVER_TYPE_WS, defaultActionTimeout), packer: packer,
		hook: hook, ctrl: new(play.InstanceCtrl), actions: make(map[string]*play.ActionUnit)}
}

func (i *wsInstance) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var conn *websocket.Conn
	var sess = play.NewSession(r.Context(), i)

	defer func() {
		if conn != nil {
			conn.Close()
		}
		recover()
	}()

	if conn, err = i.update(w, r); err != nil {
		i.hook.OnConnect(sess, err)
		return
	}

	sess.Conn.Websocket.WebsocketConn = conn
	sess.Conn.Http.Request, sess.Conn.Http.ResponseWriter = r, w

	i.accept(sess)
}

func (i *wsInstance) accept(s *play.Session) {
	var err error
	var request *play.Request

	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			fmt.Printf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
	}()

	defer func() {
		i.hook.OnClose(s, err)
	}()
	i.hook.OnConnect(s, nil)

	if request, err = i.packer.Unpack(s.Conn); request != nil {
		if err = play.DoRequest(context.Background(), s, request); err != nil {
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

			if request, err := i.packer.Unpack(sess.Conn); err != nil {
				return err
			} else {
				if err := play.DoRequest(context.Background(), sess, request); err != nil {
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

func (i *wsInstance) Info() play.IInstanceInfo {
	return i.info
}

func (i *wsInstance) Packer() play.IPacker {
	return i.packer
}

func (i *wsInstance) Transport(conn *play.Conn, data []byte) (err error) {
	if conn.Websocket.MessageType == 0 {
		conn.Websocket.MessageType = websocket.TextMessage
	}
	err = conn.Websocket.WebsocketConn.WriteMessage(conn.Websocket.MessageType, data)
	return err
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

func (i *wsInstance) Run(listener net.Listener, udplistener net.PacketConn) error {
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

func (i *wsInstance) Network() string {
	return "tcp"
}

func (i *wsInstance) ActionUnitNames() []string {
	return append([]string(nil), i.sortedNames...)
}

func (i *wsInstance) LookupActionUnit(requestName string) *play.ActionUnit {
	return i.actions[requestName]
}

func (i *wsInstance) BindActionSpace(spaceName string, actionPackages ...string) error {
	return bindActionSpace(i, spaceName, actionPackages)
}

func (i *wsInstance) UpdateActionTimeout(spaceName string, actionName string, timeout time.Duration) {
	if spaceName != "" {
		spaceName = spaceName + "."
	}
	if act := i.actions[spaceName+actionName]; act != nil {
		act.Timeout = timeout
	}
}

func (i *wsInstance) AddActionUnits(units ...*play.ActionUnit) error {
	for _, u := range units {
		if i.actions[u.RequestName] != nil {
			return errors.New("action unit " + u.RequestName + " is already exists in " + i.info.Name())
		}
		i.actions[u.RequestName] = u
		i.sortedNames = append(i.sortedNames, u.RequestName)
	}
	sort.Strings(i.sortedNames)
	return nil
}
