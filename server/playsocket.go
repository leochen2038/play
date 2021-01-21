package server

import (
	"errors"
	"fmt"
	"github.com/leochen2038/play"
	"io"
	"log"
	"net"
	"os"
	"runtime/debug"
	"syscall"
	"time"
)

var noDeadline time.Time

type PlaysocketConfig struct {
	Address   string
	DoRender  func(protocol *PlayProtocol, ctx *play.Context, err error)
	OnRequest func(ctx *play.Context) (err error)
	//ProcessFunc func(protocal *PlayProtocol)
	//ProcessChan chan *PlayProtocol
}

func BootPlaysocket(serverConfig PlaysocketConfig) {
	processFunc := func(protocol *PlayProtocol) {
		var err error
		var ctx *play.Context

		ctx = play.NewContext(play.NewInput(NewJsonParser(protocol.Message)), protocol.TagId, protocol.TraceId, protocol.SpanId, protocol.Version)
		ctx.ActionName = protocol.Action

		if serverConfig.OnRequest != nil {
			err = serverConfig.OnRequest(ctx)
		}
		if err == nil {
			err = play.RunAction(protocol.Action, ctx)
		}

		if serverConfig.DoRender != nil {
			serverConfig.DoRender(protocol, ctx, err)
		}
	}

	listen(serverConfig.Address, processFunc, nil)
}

func listen(address string, process func(protocol *PlayProtocol), channel chan *PlayProtocol) {
	var err error
	if os.Getenv(envGraceful) != "" {
		id := getGracefulSocket(1)
		if id > 2 {
			if playListener, err = net.FileListener(os.NewFile(id, "")); err != nil {
				log.Fatal("[ocket server] error inheriting socket fd")
				os.Exit(1)
			}
			if err = shouldKillParent(); err != nil {
				log.Println("[socket server] failed to close parent:", err)
				os.Exit(1)
			}
		} else {
			log.Fatal("[socket server] error socket fd < 3")
			os.Exit(1)
		}
	} else {
		if playListener, err = net.Listen("tcp", address); err != nil {
			log.Fatal("[sokcet server] listen error:", err)
			os.Exit(1)
		}
		log.Println("[sokcet server] listen success on", address)
	}
	defer playListener.Close()

	for {
		var conn net.Conn
		if conn, err = playListener.Accept(); err != nil {
			continue
		}

		log.Println("[play server]", conn.RemoteAddr().String(), "connect success")
		go accept(conn, process, channel)
	}

}

func ConnectWithPlayContext(ctx *play.Context, callerId int, address string, action string, message []byte, respond bool, timeout time.Duration) (reponseByte []byte, err error) {

	ctx.SpanId++
	var spanId = make([]byte, 0, 16)
	spanId = append(spanId, ctx.ParentSpanId...)
	spanId = append(spanId, ctx.SpanId)

	reponseByte, err = _connect(ctx.Version, address, callerId, ctx.TraceId, spanId, ctx.TagId, action, message, respond, timeout)
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) || err == io.EOF {
		// try agent
		return _connect(ctx.Version, address, callerId, ctx.TraceId, spanId, ctx.TagId, action, message, respond, timeout)
	}
	return
}

func _connect(version byte, address string, callerId int, traceId string, spanId []byte, tagId int, action string, message []byte, respond bool, timeout time.Duration) (reponseByte []byte, err error) {
	var conn *PlayConn
	if conn, err = GetSocketPoolBy(address).GetConn(); err != nil {
		return nil, fmt.Errorf("unable connect %s, %w", address, err)
	}
	defer conn.Close()

	if traceId == "" {
		traceId = play.GetMicroUqid(conn.LocalAddr().String())
	}
	requestByte, protocolSize := buildRequestBytes(version, tagId, traceId, spanId, callerId, action, message, respond)

	if n, err := conn.Write(requestByte); err != nil || n != protocolSize {
		conn.Unsable = true
		return nil, fmt.Errorf("send message error %w", err)
	}
	if respond {
		if timeout > 0 {
			conn.SetReadDeadline(time.Now().Add(timeout))
		} else {
			conn.SetReadDeadline(noDeadline)
		}

		var buffer = make([]byte, 4096)
		var surplus []byte
		var protocol *PlayProtocol
		for {
			n, err := conn.Read(buffer)
			if err != nil {
				conn.Unsable = true
				log.Println("[play server]", err, "on", conn.RemoteAddr().String())
				return nil, err
			}
			protocol, surplus, err = parseResponseProtocol(append(surplus, buffer[:n]...))
			if err != nil {
				conn.Unsable = true
				log.Println("[play server]", err, "on", conn.RemoteAddr().String())
				return nil, err
			}
			if protocol != nil {
				if protocol.TraceId != traceId {
					conn.Unsable = true
					return nil, fmt.Errorf("protocol err expect %s but %s", traceId, protocol.TraceId)
				}
				return protocol.Message, nil
			}
		}
	}

	return nil, nil
}

func accept(conn net.Conn, process func(protocol *PlayProtocol), channel chan *PlayProtocol) {
	var surplus []byte
	var protocol *PlayProtocol
	var next = true
	var buffer = make([]byte, 4096)
	defer conn.Close()

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			log.Println("[play server]", err, "on", conn.RemoteAddr().String())
			return
		}
		next = true
		surplus = append(surplus, buffer[:n]...)
		for next {
			protocol, surplus, next, err = parseRequestProtocol(surplus)
			if err != nil {
				log.Println("[play server]", err, "on", conn.RemoteAddr().String())
				return
			}
			if protocol != nil {
				wg.Add(1)
				protocol.Conn = conn
				if process != nil {
					func() {
						defer func() {
							if panicInfo := recover(); panicInfo != nil {
								log.Fatal(fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack())))
							}
						}()
						process(protocol)
					}()
				} else if channel != nil {
					channel <- protocol
				}
				wg.Done()
			}
		}
	}
}
