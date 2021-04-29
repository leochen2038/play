package clients

import (
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/middleware/golang/json"
	"log"
	"strconv"
	"unsafe"
)

type PlayClient struct {
	trace    *play.TraceContext
	ver      byte
	callerId int
	tagId    string
}

var pools *play.GroupSocket

func init() {
	pools = play.NewGroupSocket(8)
}

func NewPlayCallerWithCtx(ctx *play.Context) *PlayClient {
	return &PlayClient{trace: ctx.Trace, ver: ctx.Session.Conn.Tcp.Version, tagId: ctx.Session.Conn.Tcp.Tag, callerId: ctx.AppId}
}

func (c *PlayClient) ParseResponse(data []byte, dest interface{}) error {
	return json.Unmarshal(data, dest)
}

func (c *PlayClient) Call(service string, action string, req interface{}, respond bool) ([]byte, error) {
	var err error
	var message []byte
	var requestByte []byte
	var protocolSize int
	var conn *play.SocketConn

	if conn, err = pools.GetSocketConnByGroupName(service); err != nil {
		return nil, fmt.Errorf("unable connect %s, %w", service, err)
	}
	defer conn.Close()

	if message, err = json.Marshal(req); err != nil {
		return nil, err
	}
	switch c.ver {
	case 3:
		c.trace.SpanId++
		var spanId = make([]byte, 0, 16)
		spanId = append(spanId, c.trace.ParentSpanId...)
		spanId = append(spanId, c.trace.SpanId)
		requestByte, protocolSize = BuildTcpRequestV3(c.tagId, c.trace.TraceId, spanId, c.callerId, action, message, respond)
	default:
		requestByte, protocolSize = BuildTcpRequestV2(c.trace.TraceId, c.callerId, action, message, respond)
	}

	if n, err := conn.Conn.Write(requestByte); err != nil || n != protocolSize {
		conn.SetDead()
		return nil, fmt.Errorf("send message error %w", err)
	}

	if respond {
		var buffer = make([]byte, 4096)
		var surplus []byte
		var response []byte
		for {
			n, err := conn.Read(buffer)
			if err != nil {
				conn.SetDead()
				log.Println("[play server]", err, "on", conn.RemoteAddr().String())
				return nil, err
			}
			response, surplus, err = ReadResponseProtocol(append(surplus, buffer[:n]...))
			if err != nil {
				conn.SetDead()
				log.Println("[play server]", err, "on", conn.RemoteAddr().String())
				return nil, err
			}
			if response != nil {
				return response, nil
				//if protocol.TraceId != traceId {
				//	conn.Unsable = true
				//	return nil, fmt.Errorf("protocol err expect %s but %s", traceId, protocol.TraceId)
				//}
				//return protocol.Message, nil
			}
		}
	}
	return nil, nil
}

func BuildTcpRequestV2(traceId string, callerId int, action string, message []byte, respond bool) (buffer []byte, protocolSize int) {
	var version byte = 2
	var actionLen = byte(len(action))
	var responseByte byte = 0

	protocolSize = 49 + len(message) + len(action)
	messageSize := int2Bytes(protocolSize - 8)

	if respond {
		responseByte = 1
	}

	buffer = append(buffer, []byte("==>>")...)
	buffer = append(buffer, messageSize...)
	buffer = append(buffer, version)
	buffer = append(buffer, responseByte)
	buffer = append(buffer, ushortInt2Bytes(uint16(callerId))...)
	buffer = append(buffer, actionLen)
	buffer = append(buffer, int2Bytes(len(message))...)
	buffer = append(buffer, []byte(traceId)...)
	buffer = append(buffer, []byte(action)...)
	buffer = append(buffer, message...)

	return
}

func BuildTcpRequestV3(tag string, traceId string, spanId []byte, callerId int, action string, message []byte, respond bool) (buffer []byte, protocolSize int) {
	var actionLen = byte(len(action))
	var responseByte byte = 0
	var version byte = 3
	tagId, _ := strconv.Atoi(tag)
	protocolSize = 67 + len(action) + len(message)
	if respond {
		responseByte = 1
	}

	buffer = append(buffer, []byte("==>>")...)
	buffer = append(buffer, int2Bytes(protocolSize-8)...)
	buffer = append(buffer, version)
	buffer = append(buffer, int2Bytes(tagId)...)
	buffer = append(buffer, []byte(traceId)...)
	if len(spanId) >= 16 {
		buffer = append(buffer, spanId[:16]...)
	} else {
		buffer = append(buffer, spanId...)
		for i := 16 - len(spanId); i > 0; i-- {
			buffer = append(buffer, 0)
		}
	}

	buffer = append(buffer, int2Bytes(callerId)...)
	buffer = append(buffer, actionLen)
	buffer = append(buffer, responseByte)

	buffer = append(buffer, []byte(action)...)
	buffer = append(buffer, message...)
	return
}

func ReadResponseProtocol(buffer []byte) ([]byte, []byte, error) {
	var message []byte
	//var traceId string
	if len(buffer) < 8 {
		// log.Println("[play server] buffer byte length must > 8")
		return nil, buffer, nil
	}

	if string(buffer[:4]) != "<<==" && string(buffer[:4]) != "==>>" {
		err := fmt.Errorf("[play server] error play socket protocol head")
		return nil, nil, err
	}

	dataSize := bytes2Int(buffer[4:8]) + 8
	if dataSize > len(buffer) {
		// log.Printf("[playsocket server] error play socket protocol data length recv:%d, need:%d\n", len(buffer), dataLength)
		return nil, buffer, nil
	}

	// 检查协议标本号
	if buffer[8] != 3 && buffer[8] != 2 {
		err := fmt.Errorf("[play server] error play socket protocol version must be 2 or 3")
		return nil, nil, err
	}

	version := buffer[8]

	if version == 3 {
		//traceId = string(buffer[13:45])
		message = buffer[49:dataSize]
	} else {
		//traceId = string(buffer[13:45])
		message = buffer[45:dataSize]
	}

	return message, buffer[dataSize:], nil
}

func bytes2Int(data []byte) int {
	var ret = 0
	var l = len(data)
	var i uint = 0
	for i = 0; i < uint(l); i++ {
		ret = ret | (int(data[i]) << (i * 8))
	}
	return ret
}

func int2Bytes(data int) (ret []byte) {
	var d32 = int32(data)
	var sizeof = unsafe.Sizeof(d32)

	ret = make([]byte, sizeof)
	var tmp int32 = 0xff
	var index uint = 0
	for index = 0; index < uint(sizeof); index++ {
		ret[index] = byte((tmp << (index * 8) & d32) >> (index * 8))
	}
	return ret
}

func ushortInt2Bytes(data uint16) (ret []byte) {
	var sizeof = unsafe.Sizeof(data)
	ret = make([]byte, sizeof)
	var tmp uint16 = 0xff
	var index uint = 0
	for index = 0; index < uint(sizeof); index++ {
		ret[index] = byte((tmp << (index * 8) & data) >> (index * 8))
	}
	return ret
}
