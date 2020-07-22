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
	Address string
	Render  func(protocol *PlayProtocol, ctx *play.Context, err error)
}

func BootPlaysocket(serverConfig PlaysocketConfig) {
	listen(serverConfig.Address, func(protocol *PlayProtocol) {
		defer func() {
			if panicInfo := recover(); panicInfo != nil {
				log.Fatal(fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack())))
			}
		}()

		var err error
		ctx := play.NewContextWithInput(play.NewInput(NewJsonParser(protocol.Message)))
		err = play.RunAction(protocol.Action, ctx)
		if serverConfig.Render != nil {
			serverConfig.Render(protocol, ctx, err)
		}
	}, nil)
}

func listen(address string, process func(protocol *PlayProtocol), channel chan *PlayProtocol) {
	var err error
	if os.Getenv(envGraceful) != "" {
		id := getGracefulSocket(1)
		if id > 2 {
			if playListener, err = net.FileListener(os.NewFile(id, "")); err != nil {
				log.Fatal("[playsocket server] error inheriting socket fd")
				os.Exit(1)
			}
			if err = shouldKillParent(); err != nil {
				log.Println("[playsocket server] failed to close parent:", err)
				os.Exit(1)
			}
		} else {
			log.Fatal("[playsocket server] error socket fd < 3")
			os.Exit(1)
		}
	} else {
		if playListener, err = net.Listen("tcp", address); err != nil {
			log.Fatal("[playsokcet server] listen error:", err)
			os.Exit(1)
		}
		log.Println("[playsokcet server] listen success on", address)
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

func Connect(address string, callerId uint16, action string, message []byte, respond bool, timeout time.Duration) (reponseByte []byte, err error) {
	reponseByte, err = _connect(address, callerId, action, message, respond, timeout)
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) || err == io.EOF {
		return _connect(address, callerId, action, message, respond, timeout)
	}
	return
}

func _connect(address string, callerId uint16, action string, message []byte, respond bool, timeout time.Duration) (reponseByte []byte, err error) {
	var conn *PlayConn
	if conn, err = GetSocketPoolBy(address).GetConn(); err != nil {
		return nil, fmt.Errorf("unable connect %s, %w", address, err)
	}
	defer conn.Close()

	requestId := getMicroUqid(conn.LocalAddr().String())
	requestByte, protocolSize := buildRequest(requestId, callerId, action, message, respond)

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
			protocol, surplus, err = parseResponse(append(surplus, buffer[:n]...))
			if err != nil {
				conn.Unsable = true
				log.Println("[play server]", err, "on", conn.RemoteAddr().String())
				return nil, err
			}
			if protocol != nil {
				if protocol.requestId != requestId {
					conn.Unsable = true
					return nil, fmt.Errorf("protocol err expect %s but %s", requestId, protocol.requestId)
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

	var buffer = make([]byte, 4096)
	defer conn.Close()

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			log.Println("[play server]", err, "on", conn.RemoteAddr().String())
			return
		}
		protocol, surplus, err = parseProtocol(append(surplus, buffer[:n]...))
		if err != nil {
			log.Println("[play server]", err, "on", conn.RemoteAddr().String())
			return
		}
		if protocol != nil {
			wg.Add(1)
			protocol.Conn = conn
			if process != nil {
				process(protocol)
			} else if channel != nil {
				channel <- protocol
			}
			wg.Done()
		}
	}
}
