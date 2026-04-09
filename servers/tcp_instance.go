package servers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
)

type tcpInstance struct {
	info        play.IInstanceInfo
	hook        play.IServerHook
	ctrl        *play.InstanceCtrl
	actions     map[string]*play.ActionUnit
	sortedNames []string
	packer      play.IPacker
}

func NewTcpInstance(name string, addr string, hook play.IServerHook, packer play.IPacker, defaultActionTimeout time.Duration) *tcpInstance {
	if packer == nil {
		packer = packers.NewPlayPacker()
	}
	if hook == nil {
		hook = defaultHook{}
	}
	if defaultActionTimeout == 0 {
		defaultActionTimeout = defaultTimeout
	}
	return &tcpInstance{info: play.NewInstanceInfo(name, addr, play.SERVER_TYPE_TCP, defaultActionTimeout), packer: packer, hook: hook, ctrl: new(play.InstanceCtrl), actions: make(map[string]*play.ActionUnit)}
}

func (i *tcpInstance) onReady(s *play.Session) (err error) {
	var n int
	var buffer = make([]byte, 4096)
	var request *play.Request
	var conn = s.Conn.Tcp.Conn

	for {
		sessContext := s.Context()
		select {
		case <-sessContext.Done():
			return sessContext.Err()
		default:
			if n, err = conn.Read(buffer); err != nil {
				return
			}
			s.Conn.Tcp.Surplus = append(s.Conn.Tcp.Surplus, buffer[:n]...)
			if request, err = i.packer.Unpack(s.Conn); err != nil {
				return
			}
			if request == nil {
				continue
			} else {
				if request.Version > s.Conn.Tcp.Version {
					s.Conn.Tcp.Version = request.Version
				}
				if err = play.DoRequest(context.Background(), s, request); err != nil {
					return err
				}
			}
		}
	}
}

func (i *tcpInstance) Info() play.IInstanceInfo {
	return i.info
}

func (i *tcpInstance) Hook() play.IServerHook {
	return i.hook
}

func (i *tcpInstance) Packer() play.IPacker {
	return i.packer
}

func (i *tcpInstance) Transport(conn *play.Conn, data []byte) error {
	_, err := conn.Tcp.Conn.Write(data)
	return err
}

func (i *tcpInstance) Ctrl() *play.InstanceCtrl {
	return i.ctrl
}

func (i *tcpInstance) Run(listener net.Listener, udplistener net.PacketConn) error {
	for {
		var err error
		var conn net.Conn
		if conn, err = listener.Accept(); err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return err
			}
		}

		go func(err error, conn net.Conn) {
			s := play.NewSession(context.Background(), i)
			s.Conn.Tcp.Conn = conn

			defer func() {
				if panicInfo := recover(); panicInfo != nil {
					fmt.Printf("panic: %v\n%v", panicInfo, string(debug.Stack()))
				}
				if s.Conn.Tcp.Conn != nil {
					_ = s.Conn.Tcp.Conn.Close()
				}
			}()

			defer func() {
				i.hook.OnClose(s, err)
			}()
			i.hook.OnConnect(s, err)

			if err == nil {
				err = i.onReady(s)
			}
		}(err, conn)
	}
}

func (i *tcpInstance) Close() {
	i.ctrl.WaitTask()
}

func (i *tcpInstance) Network() string {
	return "tcp"
}

func (i *tcpInstance) ActionUnitNames() []string {
	return append([]string(nil), i.sortedNames...)
}

func (i *tcpInstance) LookupActionUnit(requestName string) *play.ActionUnit {
	return i.actions[requestName]
}

func (i *tcpInstance) BindActionSpace(spaceName string, actionPackages ...string) error {
	return bindActionSpace(i, spaceName, actionPackages)
}

func (i *tcpInstance) UpdateActionTimeout(spaceName string, actionName string, timeout time.Duration) {
	if spaceName != "" {
		spaceName = spaceName + "."
	}
	if act := i.actions[spaceName+actionName]; act != nil {
		act.Timeout = timeout
	}
}

func (i *tcpInstance) AddActionUnits(units ...*play.ActionUnit) error {
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
