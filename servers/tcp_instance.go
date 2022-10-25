package servers

import (
	"context"
	"fmt"
	"net"
	"runtime/debug"
	"strings"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
)

type TcpInstance struct {
	info   play.InstanceInfo
	hook   play.IServerHook
	ctrl   *play.InstanceCtrl
	packer play.IPacker
}

func NewTcpInstance(name string, addr string, hook play.IServerHook, packer play.IPacker) *TcpInstance {
	if packer == nil {
		packer = packers.NewPlayPacker()
	}
	if hook == nil {
		hook = defaultHook{}
	}
	return &TcpInstance{info: play.InstanceInfo{Name: name, Address: addr, Type: play.SERVER_TYPE_TCP}, packer: packer, hook: hook, ctrl: new(play.InstanceCtrl)}
}

func (i *TcpInstance) onReady(s *play.Session) (err error) {
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
			if request, err = i.packer.Receive(s.Conn); err != nil {
				return
			}
			if request == nil {
				continue
			} else {
				if request.Version > s.Conn.Tcp.Version {
					s.Conn.Tcp.Version = request.Version
				}
				if err = doRequest(context.Background(), s, request); err != nil {
					return err
				}
			}
		}
	}
}

func (i *TcpInstance) Info() play.InstanceInfo {
	return i.info
}

func (i *TcpInstance) Hook() play.IServerHook {
	return i.hook
}

func (i *TcpInstance) Packer() play.IPacker {
	return i.packer
}

func (i *TcpInstance) Transport(conn *play.Conn, data []byte) error {
	_, err := conn.Tcp.Conn.Write(data)
	return err
}

func (i *TcpInstance) Ctrl() *play.InstanceCtrl {
	return i.ctrl
}

func (i *TcpInstance) Run(listener net.Listener, udplistener net.PacketConn) error {
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

func (i *TcpInstance) Close() {
	i.ctrl.WaitTask()
}

func (i *TcpInstance) Network() string {
	return "tcp"
}
