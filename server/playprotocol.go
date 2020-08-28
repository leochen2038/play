package server

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

type PlayProtocol struct {
	version   byte
	CallerId  uint16
	requestId string
	Conn      net.Conn
	Responed  byte
	Action    string
	Message   []byte
}

func (p *PlayProtocol) ResponseMessage(message []byte) error {
	var buffer []byte
	protocolSize := 45 + len(message)
	messageSize := int2Bytes(int32(protocolSize - 8))

	buffer = append(buffer, []byte("==>>")...)
	buffer = append(buffer, messageSize...)
	buffer = append(buffer, p.version)
	buffer = append(buffer, int2Bytes(int32(len(message)))...)
	buffer = append(buffer, []byte(p.requestId)...)
	buffer = append(buffer, message...)

	n, err := p.Conn.Write(buffer)
	if err != nil || n != protocolSize {
		p.Conn.Close()
	}
	return err
}

func MarshalRequest(protocol *PlayProtocol) ([]byte, int, error) {
	actionLen := byte(len(protocol.Action))
	protocolSize := 49 + len(protocol.Message) + len(protocol.Action)
	messageSize := int2Bytes(int32(protocolSize - 8))
	buffer := make([]byte, 0, protocolSize)

	buffer = append(buffer, []byte("==>>")...)
	buffer = append(buffer, messageSize...)
	buffer = append(buffer, protocol.version)
	buffer = append(buffer, protocol.Responed)
	buffer = append(buffer, ushortInt2Bytes(protocol.CallerId)...)
	buffer = append(buffer, actionLen)
	buffer = append(buffer, int2Bytes(int32(len(protocol.Message)))...)
	buffer = append(buffer, []byte(protocol.requestId)...)
	buffer = append(buffer, []byte(protocol.Action)...)
	buffer = append(buffer, protocol.Message...)

	return buffer, protocolSize, nil
}

func ReadResponseBytes(conn net.Conn) ([]byte, error) {
	var buffer = make([]byte, 4096)
	for {
		_, err := conn.Read(buffer)
		if err != nil {
			return nil, err
		}
		if len(buffer) < 8 {
			continue
		}
		if string(buffer[:4]) != "==>>" {
			return nil, fmt.Errorf("[play server] error play socket protocol head")
		}

		dataLength := bytes2Int(buffer[4:8]) + 8
		if dataLength > len(buffer) {
			continue
		}

		if buffer[8] != 2 {
			return nil, fmt.Errorf("[play server] error play socket protocol version must be 2")
		}
		return buffer[:dataLength], nil
	}
}

func buildRequest(requestId string, callerId uint16, action string, message []byte, respond bool) (buffer []byte, protocolSize int) {

	var version byte = 2
	var actionLen byte = byte(len(action))
	var responByte byte = 0

	protocolSize = 49 + len(message) + len(action)
	messageSize := int2Bytes(int32(protocolSize - 8))
	if respond {
		responByte = 1
	}
	buffer = append(buffer, []byte("==>>")...)
	buffer = append(buffer, messageSize...)
	buffer = append(buffer, version)
	buffer = append(buffer, responByte)
	buffer = append(buffer, ushortInt2Bytes(callerId)...)
	buffer = append(buffer, actionLen)
	buffer = append(buffer, int2Bytes(int32(len(message)))...)
	buffer = append(buffer, []byte(requestId)...)
	buffer = append(buffer, []byte(action)...)
	buffer = append(buffer, message...)

	return
}

func parseResponse(buffer []byte) (*PlayProtocol, []byte, error) {
	if len(buffer) < 8 {
		log.Println("[play server] buffer byte length must > 8")
		return nil, buffer, nil
	}

	if string(buffer[:4]) != "==>>" {
		err := fmt.Errorf("[play server] error play socket protocol head")
		return nil, nil, err
	}

	dataLength := bytes2Int(buffer[4:8]) + 8
	if dataLength > len(buffer) {
		//log.Printf("[playsocket server] error play socket protocol data length recv:%d, need:%d\n", len(buffer), dataLength)
		return nil, buffer, nil
	}

	// 检查协议标本号
	if buffer[8] != 2 {
		err := fmt.Errorf("[play server] error play socket protocol version must be 2")
		return nil, nil, err
	}

	protocol := &PlayProtocol{}
	protocol.requestId = string(buffer[13:45])
	protocol.Message = buffer[45:dataLength]

	return protocol, buffer[dataLength:], nil
}

func parseProtocol(buffer []byte) (*PlayProtocol, []byte, error) {
	if len(buffer) < 8 {
		// log.Println("[play server] buffer byte length must > 8")
		return nil, buffer, nil
	}

	if string(buffer[:4]) != "==>>" {
		err := fmt.Errorf("[play server] error play socket protocol head")
		return nil, nil, err
	}

	dataLength := bytes2Int(buffer[4:8]) + 8
	if dataLength > len(buffer) {
		// log.Printf("[play server] play socket protocol data length recv:%d, need:%d\n", len(buffer), dataLength)
		return nil, buffer, nil
	}

	// 检查协议标本号
	if buffer[8] != 2 {
		err := fmt.Errorf("[play server] error play socket protocol version must be 2")
		return nil, nil, err
	}

	actionEndIdx := 49 + bytes2Int(buffer[12:13])
	messageEndIdx := actionEndIdx + bytes2Int(buffer[13:17])

	protocol := &PlayProtocol{}
	protocol.version = buffer[8]
	protocol.Responed = buffer[9]
	protocol.CallerId = uint16(bytes2Int(buffer[10:12]))
	protocol.requestId = string(buffer[17:49])
	protocol.Action = string(buffer[49:actionEndIdx])
	protocol.Message = buffer[actionEndIdx:messageEndIdx]

	return protocol, buffer[messageEndIdx:], nil
}

func bytes2Int(data []byte) int {
	var ret int = 0
	var len int = len(data)
	var i uint = 0
	for i = 0; i < uint(len); i++ {
		ret = ret | (int(data[i]) << (i * 8))
	}
	return ret
}

func int2Bytes(data int32) (ret []byte) {
	var len uintptr = unsafe.Sizeof(data)
	ret = make([]byte, len)
	var tmp int32 = 0xff
	var index uint = 0
	for index = 0; index < uint(len); index++ {
		ret[index] = byte((tmp << (index * 8) & data) >> (index * 8))
	}
	return ret
}

func ushortInt2Bytes(data uint16) (ret []byte) {
	var len uintptr = unsafe.Sizeof(data)
	ret = make([]byte, len)
	var tmp uint16 = 0xff
	var index uint = 0
	for index = 0; index < uint(len); index++ {
		ret[index] = byte((tmp << (index * 8) & data) >> (index * 8))
	}
	return ret
}

func getMicroUqid(localaddr string) string {
	var hexIp string
	ip := localaddr[:strings.Index(localaddr, ":")]
	for _, val := range strings.Split(ip, ".") {
		hex, _ := strconv.Atoi(val)
		hexIp += fmt.Sprintf("%02x", hex)
	}

	tn := time.Now()
	usec, _ := strconv.Atoi(tn.Format(".999999"))
	return strings.Replace(fmt.Sprintf("%s%06d%s%04x", tn.Format("20060102150405"), usec, hexIp, os.Getpid()%0x10000), ".", "", 1)
}
