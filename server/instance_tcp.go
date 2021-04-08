package server

import (
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
	"log"
	"net"
	"runtime/debug"
	"sync"
)

type TcpInstance struct {
	defaultRender    string
	addr             string
	name             string
	packerDelegate   play.Packer
	onAcceptHandler  func(client *play.Client) (*play.Request, error)
	onRequestHandler func(ctx *play.Context) error
	renderHandler  func(ctx *play.Context)
	wg               sync.WaitGroup
}

func NewSocketInstance(name string, addr string, packer play.Packer, render func(ctx *play.Context)) *TcpInstance {
	i := &TcpInstance{name: name, addr:addr}
	if packer != nil {
		i.packerDelegate = packer
	} else {
		i.packerDelegate = new(packers.TcpPlayPacker)
	}
	if render != nil {
		i.renderHandler = render
	} else {
		i.renderHandler = func(ctx *play.Context) {
			_ = ctx.Session.Write(ctx.Output)
		}
	}
	return i
}

func (i *TcpInstance)accept(conn net.Conn) {
	var err error
	var request *play.Request
	var c = new(play.Client)
	var s = play.NewSession(c, i.packerDelegate)
	c.Tcp.Conn = conn

	defer func() {
		_ = conn.Close()
		if panicInfo := recover(); panicInfo != nil {
			log.Fatal(fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack())))
		}
	}()

	fmt.Println("new connect:", conn.RemoteAddr())
	if i.onAcceptHandler != nil {
		if request, err = i.onAcceptHandler(c); err != nil {
			fmt.Println("accept:", err)
			return
		}
		if request != nil {
			doRequest(i, s, request)
		}
	}
	i.onReady(s)
}

func (i *TcpInstance)onReady(s *play.Session) {
	var err error
	var surplus []byte
	var buffer = make([]byte, 4096)
	var n int
	var request *play.Request
	var conn = s.Client.Tcp.Conn


	for {
		if n, err = conn.Read(buffer); err != nil {
			log.Println("[play server]", err, "on", conn.RemoteAddr().String())
			return
		}
		surplus = append(surplus, buffer[:n]...)
		if true {
			if request, surplus, err = i.packerDelegate.Read(s.Client, surplus); err != nil {
				log.Println("[play server]", err, "on", conn.RemoteAddr().String())
				return
			}
			if request == nil {
				continue
			} else {
				s.Client.Tcp.Tag = request.Tag
				s.Client.Tcp.TraceId = request.TraceId
				s.Client.Tcp.Version = request.Version
				i.wg.Add(1)
				doRequest(i, s, request)
				i.wg.Done()
			}
		}
	}
}

// 实现 server接口
func (i *TcpInstance)SetOnRequestHandler(handler func(ctx *play.Context) error) {
	i.onRequestHandler = handler
}
func (i *TcpInstance)SetRenderHandler(handler func(ctx *play.Context)) {
	i.renderHandler = handler
}
func (i *TcpInstance)SetOnAcceptHandler(handler func(c *play.Client) (*play.Request, error)) {
	i.onAcceptHandler = handler
}
func (i *TcpInstance)OnRequest(ctx *play.Context) error {
	if i.onRequestHandler != nil {
		return i.onRequestHandler(ctx)
	}
	return nil
}

func (i *TcpInstance)Render(ctx *play.Context) {
	if i.renderHandler != nil {
		i.renderHandler(ctx)
	}
}

func (i *TcpInstance)Address() string {
	return i.addr
}

func (i *TcpInstance)Name() string {
	return i.name
}

func (i *TcpInstance)Type() int {
	return TypeTcp
}

func (i *TcpInstance)SetPackerDelegate(delegate play.Packer) {
	if delegate != nil {
		i.packerDelegate = delegate
	}
}

func (i *TcpInstance)Run(listener net.Listener) error {
	for {
		if conn, err := listener.Accept(); err != nil {
			fmt.Println("connect error:", err)
			continue
		} else {
			go i.accept(conn)
		}
	}
}

func (i *TcpInstance)Close() {
	i.wg.Wait()
}

func (i *TcpInstance) Packer() play.Packer {
	return i.packerDelegate
}