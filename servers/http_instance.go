package servers

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
)

type httpInstance struct {
	info        play.IInstanceInfo
	hook        play.IServerHook
	ctrl        *play.InstanceCtrl
	packer      play.IPacker
	actions     map[string]*play.ActionUnit
	sortedNames []string
	tlsConfig   *tls.Config
	httpServer  http.Server
	ws          *wsInstance
	sse         *sseInstance
	h2c         *h2cInstance
	mcp         *mcpInstance
}

func NewHttpInstance(name string, addr string, hook play.IServerHook, packer play.IPacker, defaultActionTimeout time.Duration) *httpInstance {
	if packer == nil {
		packer = packers.NewHttpPacker()
	}
	if hook == nil {
		hook = defaultHook{}
	}
	if defaultActionTimeout == 0 {
		defaultActionTimeout = defaultTimeout
	}
	return &httpInstance{info: play.NewInstanceInfo(name, addr, play.SERVER_TYPE_HTTP, defaultActionTimeout), packer: packer, hook: hook, ctrl: new(play.InstanceCtrl), actions: make(map[string]*play.ActionUnit)}
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

func (i *httpInstance) Info() play.IInstanceInfo {
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
			sess.Conn.Type = play.SERVER_TYPE_WS
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

	if i.mcp != nil && i.mcp.httpHandler != nil && strings.HasPrefix(r.URL.Path, "/mcp") {
		i.mcp.httpHandler.ServeHTTP(w, r)
		return
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
	if request, err = i.packer.Unpack(sess.Conn); err != nil {
		return
	}
	err = play.DoRequest(r.Context(), sess, request)
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

func (i *httpInstance) SetMCPInstance(m *mcpInstance) {
	i.mcp = m
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

func (i *httpInstance) ActionUnits() map[string]*play.ActionUnit {
	return i.actions
}

func (i *httpInstance) ActionUnitNames() []string {
	return append([]string(nil), i.sortedNames...)
}

func (i *httpInstance) LookupActionUnit(requestName string) *play.ActionUnit {
	return i.actions[requestName]
}

func (i *httpInstance) BindActionSpace(spaceName string, actionPackages ...string) error {
	return bindActionSpace(i, spaceName, actionPackages)
}

func (i *httpInstance) UpdateActionTimeout(spaceName string, actionName string, timeout time.Duration) {
	if spaceName != "" {
		spaceName = spaceName + "."
	}
	if act := i.actions[spaceName+actionName]; act != nil {
		act.Timeout = timeout
	}
}

func (i *httpInstance) AddActionUnits(units ...*play.ActionUnit) error {
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
