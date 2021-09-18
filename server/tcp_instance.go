package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/leochen2038/play"
	"net"
	"runtime/debug"
)

type TcpInstance struct {
	info      play.InstanceInfo
	hook      play.IServerHook
	ctrl      *play.InstanceCtrl
	transport play.ITransport
}

func NewTcpInstance(name string, addr string, transport play.ITransport, hook play.IServerHook) (*TcpInstance, error) {
	if transport == nil {
		return nil, errors.New("tcp instance transport must not be nil")
	}
	if hook == nil {
		return nil, errors.New("tcp instance server hook must not be nil")
	}
	return &TcpInstance{info: play.InstanceInfo{Name: name, Address: addr, Type: TypeTcp}, transport: transport, hook: hook, ctrl: new(play.InstanceCtrl)}, nil
}

func (i *TcpInstance) accept(s *play.Session) {
	var err error
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
		_ = s.Conn.Tcp.Conn.Close()
		i.hook.OnClose(s, err)
	}()

	i.hook.OnConnect(s, nil)
	err = i.onReady(s)
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
			if true {
				if request, err = i.transport.Receive(s.Conn); err != nil {
					return
				}
				if request == nil {
					continue
				} else {
					s.Conn.Tcp.Version = request.Version
					if err = doRequest(s, request); err != nil {
						return err
					}
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

func (i *TcpInstance) Transport() play.ITransport {
	return i.transport
}

func (i *TcpInstance) Ctrl() *play.InstanceCtrl {
	return i.ctrl
}

func (i *TcpInstance) Run(listener net.Listener) error {
	for {
		if conn, err := listener.Accept(); err != nil {
			i.hook.OnConnect(play.NewSession(context.Background(), nil, i), err)
			continue
		} else {
			go func() {
				s := play.NewSession(context.Background(), new(play.Conn), i)
				s.Conn.Tcp.Conn = conn
				i.accept(s)
			}()
		}
	}
}

func (i *TcpInstance) Close() {
	i.ctrl.WaitTask()
}
