package server

import (
	"errors"
	"fmt"
	"net"
	"time"
	"unsafe"
)

// request  protocol
// 4  byte : ==>>
// 4  byte : dataSize
// 1  byte : version
// 4  byte : tagId
// 32 byte : traceId // 45

// 16 byte : spanId // 61
// 4  byte : callId
// 1  byte : actionLen
// 1  byte : responed
// total   : 67 bytes
// action, data

// response protocol
// 4  byte : <<==
// 4  byte : dataSize
// 1  byte : version
// 4  byte : tagId
// 32 byte : traceId // 45

// 4  byte : rc
// total   : 49 bytes
// data

type PlayProtocol struct {
	Rc       int
	Version  byte
	CallerId int
	TagId    int
	TraceId  string
	SpanId   []byte
	Conn     net.Conn
	Respond  byte
	Action   string
	Message  []byte
}

func (p *PlayProtocol) ResponseByMessage(message []byte, rc int) error {
	var buffer []byte
	if p.Version == 3 {
		protocolSize := 49 + len(message)

		buffer = append(buffer, []byte("<<==")...)
		buffer = append(buffer, int2Bytes(protocolSize-8)...)
		buffer = append(buffer, p.Version)
		buffer = append(buffer, int2Bytes(p.TagId)...)
		buffer = append(buffer, []byte(p.TraceId)...)

		buffer = append(buffer, int2Bytes(rc)...)
		buffer = append(buffer, message...)

		n, err := p.Conn.Write(buffer)
		if err != nil || n != protocolSize {
			return p.Conn.Close()
		}
		return err
	} else {
		protocolSize := 45 + len(message)
		messageSize := int2Bytes(protocolSize - 8)

		buffer = append(buffer, []byte("==>>")...)
		buffer = append(buffer, messageSize...)
		buffer = append(buffer, p.Version)
		buffer = append(buffer, int2Bytes(len(message))...)
		buffer = append(buffer, []byte(p.TraceId)...)
		buffer = append(buffer, message...)

		n, err := p.Conn.Write(buffer)
		if err != nil || n != protocolSize {
			p.Conn.Close()
		}
		return err
	}
}

func MarshalRequest(protocol *PlayProtocol) ([]byte, int, error) {
	protocolSize := 67 + len(protocol.Action) + len(protocol.Message)
	buffer := make([]byte, 0, protocolSize)

	buffer = append(buffer, []byte("==>>")...)
	buffer = append(buffer, int2Bytes(protocolSize-8)...)
	buffer = append(buffer, protocol.Version)
	buffer = append(buffer, int2Bytes(protocol.TagId)...)
	buffer = append(buffer, []byte(protocol.TraceId)...)
	if len(protocol.SpanId) >= 16 {
		buffer = append(buffer, protocol.SpanId[:16]...)
	} else {
		buffer = append(buffer, protocol.SpanId...)
		for i := 16 - len(protocol.SpanId); i > 0; i-- {
			buffer = append(buffer, 0)
		}
	}

	buffer = append(buffer, int2Bytes(protocol.CallerId)...)
	buffer = append(buffer, byte(len(protocol.Action)))
	buffer = append(buffer, protocol.Respond)

	buffer = append(buffer, []byte(protocol.Action)...)
	buffer = append(buffer, protocol.Message...)

	return buffer, protocolSize, nil
}

func UnmarshalResponse(buffer []byte, protocol *PlayProtocol) error {
	if protocol == nil {
		return errors.New("dest protocol is nil")
	}
	dataLength := bytes2Int(buffer[4:8]) + 8
	protocol.Version = buffer[8]
	protocol.TagId = bytes2Int(buffer[9:13])
	protocol.TraceId = string(buffer[13:45])
	protocol.Rc = bytes2Int(buffer[45:49])
	protocol.Message = buffer[49:dataLength]
	return nil
}

func ReadResponseBytes(conn net.Conn, timeout time.Duration) ([]byte, error) {
	var buffer = make([]byte, 4096)
	var catchBytes []byte
	if timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(timeout))
	} else {
		conn.SetReadDeadline(noDeadline)
	}

	for {
		_, err := conn.Read(buffer)
		if err != nil {
			return nil, err
		}

		catchBytes = append(catchBytes, buffer...)
		if len(catchBytes) < 8 {
			continue
		}

		dataLength := bytes2Int(catchBytes[4:8]) + 8
		if dataLength > len(catchBytes) {
			continue
		}

		if string(catchBytes[:4]) != "<<==" {
			return nil, fmt.Errorf("[play server] error play socket protocol head")
		}

		if catchBytes[8] != 3 {
			return nil, fmt.Errorf("[play server] error play socket protocol version must be 3")
		}

		return catchBytes[:dataLength], nil
	}
}

func buildRequestBytes(version byte, tagId int, traceId string, spanId []byte, callerId int, action string, message []byte, respond bool) (buffer []byte, protocolSize int) {
	if version == 3 {
		var actionLen = byte(len(action))
		var responByte byte = 0

		protocolSize = 67 + len(action) + len(message)
		if respond {
			responByte = 1
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
		buffer = append(buffer, responByte)

		buffer = append(buffer, []byte(action)...)
		buffer = append(buffer, message...)
	} else {
		var version byte = 2
		var actionLen byte = byte(len(action))
		var responByte byte = 0

		protocolSize = 49 + len(message) + len(action)
		messageSize := int2Bytes(protocolSize - 8)
		if respond {
			responByte = 1
		}
		buffer = append(buffer, []byte("==>>")...)
		buffer = append(buffer, messageSize...)
		buffer = append(buffer, version)
		buffer = append(buffer, responByte)
		buffer = append(buffer, ushortInt2Bytes(uint16(callerId))...)
		buffer = append(buffer, actionLen)
		buffer = append(buffer, int2Bytes(len(message))...)
		buffer = append(buffer, []byte(traceId)...)
		buffer = append(buffer, []byte(action)...)
		buffer = append(buffer, message...)
	}

	return
}

func parseResponseProtocol(buffer []byte) (*PlayProtocol, []byte, error) {
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

	protocol := &PlayProtocol{}
	protocol.Version = buffer[8]

	if protocol.Version == 3 {
		protocol.TagId = bytes2Int(buffer[9:13])
		protocol.TraceId = string(buffer[13:45])
		protocol.Rc = bytes2Int(buffer[45:49])
		protocol.Message = buffer[49:dataSize]
	} else {
		protocol.TraceId = string(buffer[13:45])
		protocol.Message = buffer[45:dataSize]
	}

	return protocol, buffer[dataSize:], nil
}

func parseRequestProtocol(buffer []byte) (*PlayProtocol, []byte, bool, error) {
	if len(buffer) < 8 {
		// log.Println("[play server] buffer byte length must > 8")
		return nil, buffer, false, nil
	}

	if string(buffer[:4]) != "==>>" {
		err := fmt.Errorf("[play server] error play socket protocol head")
		return nil, nil, false, err
	}

	dataLength := bytes2Int(buffer[4:8]) + 8
	if dataLength > len(buffer) {
		// log.Printf("[play server] play socket protocol data length recv:%d, need:%d\n", len(buffer), dataLength)
		return nil, buffer, false, nil
	}

	// 检查协议标本号
	if buffer[8] != 3 && buffer[8] != 2 {
		err := fmt.Errorf("[play server] error play socket protocol version must be 2 or 3")
		return nil, nil, false, err
	}

	protocol := &PlayProtocol{}
	protocol.Version = buffer[8]
	if protocol.Version == 3 {
		protocol.TagId = bytes2Int(buffer[9:13])
		protocol.TraceId = string(buffer[13:45])
		for _, v := range buffer[45:61] {
			if v > 0 {
				protocol.SpanId = append(protocol.SpanId, v)
			}
		}

		protocol.CallerId = bytes2Int(buffer[61:65])
		actionEndIdx := 67 + bytes2Int(buffer[65:66])
		protocol.Respond = buffer[66]

		protocol.Action = string(buffer[67:actionEndIdx])
		protocol.Message = buffer[actionEndIdx:dataLength]
	} else {
		actionEndIdx := 49 + bytes2Int(buffer[12:13])
		messageEndIdx := actionEndIdx + bytes2Int(buffer[13:17])
		protocol.Respond = buffer[9]
		protocol.CallerId = bytes2Int(buffer[10:12])
		protocol.TraceId = string(buffer[17:49])
		protocol.Action = string(buffer[49:actionEndIdx])
		protocol.Message = buffer[actionEndIdx:messageEndIdx]
	}

	//fmt.Println(protocol.SpanId)
	return protocol, buffer[dataLength:], true, nil
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
