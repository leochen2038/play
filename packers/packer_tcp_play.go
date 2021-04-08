package packers

import (
	"errors"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/middleware/golang/json"
	"github.com/leochen2038/play/parsers"
	"strconv"
	"unsafe"
)

// request  protocol v3
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
// action, json

// response protocol v3
// 4  byte : <<==
// 4  byte : dataSize
// 1  byte : version
// 4  byte : tagId
// 32 byte : traceId // 45
// 4  byte : rc
// total   : 49 bytes
// json

// request protocol v4
// 4  byte : ==>>
// 4  byte : dataSize
// 1  byte : version
// 1  byte : tag长度
// 1  byte : traceId长度
// 1  byte : spanId长度
// 1  byte : caller长度
// 1  byte : action长度
// 1  byte : respond 0:不需要响应, 1:需要响应
// 1  byte : protocol 0:二进制, 1:json

// response protocol v4
// 4  byte : ==>>
// 4  byte : dataSize
// 1  byte : version
// 1  byte : tag长度
// 1  byte : traceId长度
// 1  byte : spanId长度
// 4  byte : rc状态码
// 1  byte : protocol 0:二进制, 1:json



type TcpPlayPacker struct {

}


func (p *TcpPlayPacker)Read(c *play.Client, buffer []byte) (*play.Request, []byte, error) {
	var err error
	if len(buffer) < 8 {
		return nil, buffer, nil
	}
	if string(buffer[:4]) != "==>>" {
		return nil, nil, errors.New("socket protocol head error")
	}

	dataSize := bytes2Int(buffer[4:8]) + 8
	if len(buffer) < dataSize {
		return nil, buffer, nil
	}

	request := play.Request{Version: buffer[8]}
	switch request.Version {
	case 2: err = readProtocolV2(buffer, dataSize, &request)
	case 3: err = readProtocolV3(buffer, dataSize, &request)
	default:
		err = errors.New("socket protocol version error")
	}
	if err != nil {
		return nil, nil, err
	}
	return &request, buffer[dataSize:], nil
}

func (p *TcpPlayPacker) Write(c *play.Client, output play.Output) (err error) {
	var message []byte
	var buffer []byte

	if message, err = json.Marshal(output.All()); err != nil {
		return err
	}

	switch c.Tcp.Version {
	case 2: buffer = packProtocolV2(message, c.Tcp.TraceId)
	case 3:
		irc, rc := output.Get("rc"), 0
		if output.Get("rc") != nil {
			rc = irc.(int)
		}
		tagId, _ := strconv.Atoi(c.Tcp.Tag)
		buffer = packProtocolV3(message, c.Tcp.TraceId, rc, tagId)
	}

	n, err := c.Tcp.Conn.Write(buffer)
	if err != nil || n != len(buffer) {
		_ = c.Tcp.Conn.Close()
	}

	return
}

func packProtocolV2(message []byte, traceId string) (buffer []byte) {
	protocolSize := 45 + len(message)
	messageSize := int2Bytes(protocolSize - 8)

	buffer = append(buffer, []byte("==>>")...)
	buffer = append(buffer, messageSize...)
	buffer = append(buffer, byte(2))
	buffer = append(buffer, int2Bytes(len(message))...)
	buffer = append(buffer, []byte(traceId)...)
	buffer = append(buffer, message...)

	return
}

func packProtocolV3(message []byte, traceId string, rc int, tagId int)(buffer []byte) {
	protocolSize := 49 + len(message)

	buffer = append(buffer, []byte("<<==")...)
	buffer = append(buffer, int2Bytes(protocolSize-8)...)
	buffer = append(buffer, byte(3))
	buffer = append(buffer, int2Bytes(tagId)...)
	buffer = append(buffer, []byte(traceId)...)

	buffer = append(buffer, int2Bytes(rc)...)
	buffer = append(buffer, message...)
	return
}


func readProtocolV2(buffer []byte, dataSize int, protocol *play.Request) error {
	actionEndIdx := 49 + bytes2Int(buffer[12:13])
	messageEndIdx := actionEndIdx + bytes2Int(buffer[13:17])
	if buffer[9] > 0 {
		protocol.Respond = true
	}
	protocol.Caller = strconv.Itoa(bytes2Int(buffer[10:12]))

	protocol.TraceId = string(buffer[17:49])
	protocol.ActionName = string(buffer[49:actionEndIdx])

	protocol.Parser = parsers.NewJsonParser(buffer[actionEndIdx:messageEndIdx])
	protocol.Format = "json"

	return nil
}

func readProtocolV3(buffer []byte, dataSize int, protocol *play.Request) error {
	if dataSize < 67 {
		return errors.New("socket protocol format error")
	}
	protocol.Tag = strconv.Itoa(bytes2Int(buffer[9:13]))
	protocol.TraceId = string(buffer[13:45])

	for _, v := range buffer[45:61] {
		if v > 0 {
			protocol.SpanId = append(protocol.SpanId, v)
		}
	}

	protocol.Caller = strconv.Itoa(bytes2Int(buffer[61:65]))
	actionEndIdx := 67 + bytes2Int(buffer[65:66])
	if buffer[66] > 0 {
		protocol.Respond = true
	}

	if dataSize < actionEndIdx || actionEndIdx < 67 {
		return errors.New("socket protocol length error")
	}

	protocol.ActionName = string(buffer[67:actionEndIdx])
	protocol.Parser = parsers.NewJsonParser(buffer[actionEndIdx:dataSize])
	protocol.Format = "json"

	return nil
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