package server

import (
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
	"log"
	"net"
	"runtime/debug"
	"sync"
	"time"
)

type TcpInstance struct {
	defaultRender    string
	addr             string
	name             string
	inputMaxSize     int64
	packerDelegate   play.Packer
	onAcceptHandler  func(client *play.Client) (*play.Request, error)
	onRequestHandler func(ctx *play.Context) error
	onResponseHandler  func(ctx *play.Context) error
	wg               sync.WaitGroup
	requestTimeout   time.Duration
}



func (i *TcpInstance)SetOnRequestHandler(handler func(ctx *play.Context) error) {
	i.onRequestHandler = handler
}
func (i *TcpInstance)SetOnRenderHandler(handler func(ctx *play.Context) error) {
	i.onResponseHandler = handler
}


func NewSocketInstance(name string, addr string) *TcpInstance {
	i := &TcpInstance{name: name, addr:addr, packerDelegate: new(packers.TcpPlayPacker)}
	return i
}

func (i *TcpInstance)accept(conn net.Conn) {
	var err error
	var request *play.Request
	var c = new(play.Client)
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
			doRequest(i, c, request)
		}
	}
	i.readyToRead(c)
}

func (i *TcpInstance)readyToRead(c *play.Client) {
	var err error
	var surplus []byte
	var buffer = make([]byte, 4096)
	var n int
	var request *play.Request
	var conn = c.Tcp.Conn

	for {
		if n, err = conn.Read(buffer); err != nil {
			log.Println("[play server]", err, "on", conn.RemoteAddr().String())
			return
		}
		surplus = append(surplus, buffer[:n]...)
		if true {
			if request, surplus, err = i.packerDelegate.Read(c, surplus); err != nil {
				log.Println("[play server]", err, "on", conn.RemoteAddr().String())
				return
			}
			if request == nil {
				continue
			} else {
				i.wg.Add(1)
				doRequest(i, c, request)
				i.wg.Done()
			}
		}
	}
}

// 实现 server接口
func (i *TcpInstance)InputMaxSize() int64 {
	return i.inputMaxSize
}

func (i *TcpInstance)RequestTimeout() time.Duration {
	return i.requestTimeout
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

func (i *TcpInstance)OnResponse(ctx *play.Context) error {
	if i.onResponseHandler != nil {
		return i.onResponseHandler(ctx)
	}
	return nil
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

func (i *TcpInstance)DefaultRender() string {
	return i.defaultRender
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