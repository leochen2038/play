package client

import (
	"fmt"
	"github.com/leochen2038/play"
	"log"
	"net"
	"time"
)

var noDeadline time.Time

func RequestWithPlayTrace(version byte, trace *play.TraceContext, callerId int, address string, action string, message []byte, respond bool, timeout time.Duration) (reponseByte []byte, err error) {
	trace.SpanId++
	var spanId = make([]byte, 0, 16)
	spanId = append(spanId, trace.ParentSpanId...)
	spanId = append(spanId, trace.SpanId)

	return _connect(version, address, callerId, trace.TraceId, spanId, trace.TagId, action, message, respond, timeout)
}

func _connect(version byte, address string, callerId int, traceId string, spanId []byte, tagId int, action string, message []byte, respond bool, timeout time.Duration) (reponseByte []byte, err error) {
	var conn *PlayConn
	if conn, err = GetSocketPoolBy(address).GetConn(); err != nil {
		return nil, fmt.Errorf("unable connect %s, %w", address, err)
	}
	defer conn.Close()

	if traceId == "" {
		traceId = play.Generate28Id("trac", "", net.ParseIP(conn.LocalAddr().String()))
	}
	requestByte, protocolSize := buildRequestBytes(version, tagId, traceId, spanId, callerId, action, message, respond)

	if n, err := conn.Write(requestByte); err != nil || n != protocolSize {
		conn.Unsable = true
		return nil, fmt.Errorf("send message error %w, send:%d, protocolSize:%d", err, n, protocolSize)
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
