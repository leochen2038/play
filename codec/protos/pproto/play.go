package pproto

import (
	"errors"
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
// 1  byte : action长度
// 1  byte : respond 0:不需要响应, 1:需要响应
// 1  byte : render 0:json
// 1  byte : traceId长度
// 1  byte : spanId长度
// 4  byte : callerId
// 4  byte : tagId
// 4  byte : header长度
// 4  byte : body长度

// response protocol v4
// 4  byte : <<==
// 4  byte : dataSize
// 1  byte : version
// 1  byte : render 0:json
// 1  byte : traceId长度
// 4  byte : rc错误码
// 4  byte : body长度

type PlayProtocolRequest struct {
	Version    byte
	Action     string
	NonRespond bool
	TraceId    string
	SpanId     []byte
	CallerId   int
	TagId      int
	Header     []byte
	Body       []byte
	Render     byte
}

type PlayProtocolResponse struct {
	Version    byte
	TraceId    string
	TagId      int
	ResultCode int
	Body       []byte
	Render     byte
}

// PackRequest 封包request请求
func MarshalProtocolRequest(request PlayProtocolRequest) ([]byte, error) {
	var err error
	buffer := []byte("==>>")
	switch request.Version {
	case 2:
		buffer, err = _packRequestV2(request, buffer)
	case 3:
		buffer, err = _packRequestV3(request, buffer)
	default:
		buffer, err = _packRequestV4(request, buffer)
	}

	return buffer, err
}

func _packRequestV2(request PlayProtocolRequest, buffer []byte) ([]byte, error) {
	if len(request.Action) > 255 {
		return nil, errors.New("action length error")
	}
	if len(request.TraceId) > 32 {
		return nil, errors.New("traceId length error")
	}
	if request.CallerId > 0xffff {
		return nil, errors.New("callerId length error")
	}

	buffer = append(buffer, _intToBytes(41+len(request.Body)+len(request.Action))...)
	buffer = append(buffer, byte(2))
	buffer = append(buffer, _nonRespondToByte(request.NonRespond))
	buffer = append(buffer, _uint16ToBytes(uint16(request.CallerId))...)
	buffer = append(buffer, byte(len(request.Action)))
	buffer = append(buffer, _intToBytes(len(request.Body))...)

	buffer = append(buffer, []byte(request.TraceId)...)
	buffer = append(buffer, []byte(request.Action)...)
	buffer = append(buffer, request.Body...)

	return buffer, nil
}

func _packRequestV3(request PlayProtocolRequest, buffer []byte) ([]byte, error) {
	if len(request.Action) > 255 {
		return nil, errors.New("action length error")
	}
	if len(request.TraceId) > 32 {
		return nil, errors.New("traceId length error")
	}
	if len(request.SpanId) > 16 {
		return nil, errors.New("spanId length error")
	}

	buffer = append(buffer, _intToBytes(59+len(request.Action)+len(request.Body))...)
	buffer = append(buffer, byte(3))
	buffer = append(buffer, _intToBytes(request.TagId)...)
	buffer = append(buffer, []byte(request.TraceId)...)

	buffer = append(buffer, request.SpanId...)
	for i := 16 - len(request.SpanId); i > 0; i-- {
		buffer = append(buffer, 0)
	}

	buffer = append(buffer, _intToBytes(request.CallerId)...)
	buffer = append(buffer, byte(len(request.Action)))
	buffer = append(buffer, _nonRespondToByte(request.NonRespond))

	buffer = append(buffer, []byte(request.Action)...)
	buffer = append(buffer, request.Body...)

	return buffer, nil
}

func _packRequestV4(request PlayProtocolRequest, buffer []byte) ([]byte, error) {
	if len(request.Action) > 255 {
		return nil, errors.New("action length error")
	}
	if len(request.TraceId) > 255 {
		return nil, errors.New("traceId length error")
	}
	if len(request.SpanId) > 255 {
		return nil, errors.New("spanId length error")
	}

	var dataSize = 22 + len(request.Action) +
		len(request.TraceId) +
		len(request.SpanId) +
		len(request.Header) +
		len(request.Body)

	buffer = append(buffer, _intToBytes(dataSize)...)
	buffer = append(buffer, byte(4))

	buffer = append(buffer, uint8(len(request.Action)))
	buffer = append(buffer, boolTobyte(request.NonRespond))
	buffer = append(buffer, request.Render)
	buffer = append(buffer, uint8(len(request.TraceId)))
	buffer = append(buffer, uint8(len(request.SpanId)))

	buffer = append(buffer, _intToBytes(request.CallerId)...)
	buffer = append(buffer, _intToBytes(request.TagId)...)

	buffer = append(buffer, _intToBytes(len(request.Header))...)
	buffer = append(buffer, _intToBytes(len(request.Body))...)

	buffer = append(buffer, []byte(request.Action)...)
	buffer = append(buffer, []byte(request.TraceId)...)
	buffer = append(buffer, request.SpanId...)
	buffer = append(buffer, request.Header...)
	buffer = append(buffer, request.Body...)

	return buffer, nil
}

// UnpackRequest 解包request请求
func UnmarshalProtocolRequest(data []byte) (protocol PlayProtocolRequest, dataSize uint32, err error) {
	if len(data) < 9 {
		return
	}

	if string(data[:4]) != "==>>" {
		return protocol, 0, errors.New("socket protocol head error")
	}

	size := _bytesToUint32(data[4:8]) + 8
	if uint32(len(data)) < size {
		return
	}

	dataSize = size
	protocol.Version = _bytesToUint8(data[8:9])
	switch protocol.Version {
	case 2:
		err = _unpackRequestV2(data, dataSize, &protocol)
	case 3:
		err = _unpackRequestV3(data, dataSize, &protocol)
	case 4:
		err = _unpackRequestV4(data, dataSize, &protocol)
	default:
		err = errors.New("socket protocol version error")
	}
	return
}

func _unpackRequestV2(buffer []byte, dataSize uint32, protocol *PlayProtocolRequest) (err error) {
	actionEndIdx := 49 + _bytesToInt(buffer[12:13])
	messageEndIdx := actionEndIdx + _bytesToInt(buffer[13:17])

	if buffer[9] > 0 {
		protocol.NonRespond = false
	}

	protocol.CallerId = _bytesToInt(buffer[10:12])
	protocol.TraceId = string(buffer[17:49])
	protocol.Action = string(buffer[49:actionEndIdx])
	protocol.Body = buffer[actionEndIdx:messageEndIdx]

	return
}

func _unpackRequestV3(buffer []byte, dataSize uint32, protocol *PlayProtocolRequest) (err error) {
	protocol.TagId = _bytesToInt(buffer[9:13])
	protocol.TraceId = string(buffer[13:45])

	for _, v := range buffer[45:61] {
		if v == 0 {
			break
		}
		protocol.SpanId = append(protocol.SpanId, v)
	}

	protocol.CallerId = _bytesToInt(buffer[61:65])
	actionEndIdx := 67 + _bytesToInt(buffer[65:66])

	if buffer[66] > 0 {
		protocol.NonRespond = false
	}
	if uint32(actionEndIdx) > dataSize {
		return errors.New("socket protocol format error")
	}

	protocol.Action = string(buffer[67:actionEndIdx])
	protocol.Body = buffer[actionEndIdx:dataSize]

	return
}

func _unpackRequestV4(buffer []byte, dataSize uint32, protocol *PlayProtocolRequest) (err error) {

	actionLength := _bytesToUint8(buffer[9:10])
	respond := _bytesToUint8(buffer[10:11])
	protocol.Render = _bytesToUint8(buffer[11:12])
	traceIdLength := uint32(_bytesToUint8(buffer[12:13]))
	spanIdLength := uint32(_bytesToUint8(buffer[13:14]))
	protocol.TagId = _bytesToInt(buffer[14:18])
	protocol.CallerId = _bytesToInt(buffer[18:22])
	headerLength := _bytesToUint32(buffer[22:26])
	bodyLength := _bytesToUint32(buffer[26:30])

	var idx uint32 = 30
	protocol.Action = string(buffer[idx : idx+uint32(actionLength)])
	idx += uint32(actionLength)

	if respond > 0 {
		protocol.NonRespond = true
	}
	if traceIdLength > 0 {
		protocol.TraceId = string(buffer[idx : idx+traceIdLength])
		idx += traceIdLength
	}
	if spanIdLength > 0 {
		protocol.SpanId = make([]byte, spanIdLength)
		copy(protocol.SpanId, buffer[idx:idx+spanIdLength])
		idx += spanIdLength
	}

	if headerLength > 0 {
		idx += headerLength
	}
	if bodyLength > 0 {
		protocol.Body = make([]byte, bodyLength)
		copy(protocol.Body, buffer[idx:idx+bodyLength])
		idx += bodyLength
	}

	return
}

func MarshalProtocolResponse(response PlayProtocolResponse) ([]byte, error) {
	buffer := []byte("<<==")

	switch response.Version {
	case 2:
		buffer = []byte("==>>")
		return _packResponseV2(response, buffer)
	case 3:
		return _packResponseV3(response, buffer)
	default:
		return _packResponseV4(response, buffer)
	}
}

func _packResponseV2(response PlayProtocolResponse, buffer []byte) ([]byte, error) {
	protocolSize := 45 + len(response.Body)
	messageSize := _intToBytes(protocolSize - 8)

	buffer = append(buffer, messageSize...)
	buffer = append(buffer, byte(2))
	buffer = append(buffer, _intToBytes(len(response.Body))...)
	buffer = append(buffer, []byte(response.TraceId)...)
	buffer = append(buffer, response.Body...)

	return buffer, nil
}

func _packResponseV3(response PlayProtocolResponse, buffer []byte) ([]byte, error) {
	protocolSize := 49 + len(response.Body)

	buffer = append(buffer, _intToBytes(protocolSize-8)...)
	buffer = append(buffer, byte(3))
	buffer = append(buffer, _intToBytes(response.TagId)...)
	buffer = append(buffer, []byte(response.TraceId)...)

	buffer = append(buffer, _intToBytes(response.ResultCode)...)
	buffer = append(buffer, response.Body...)

	return buffer, nil
}

func _packResponseV4(response PlayProtocolResponse, buffer []byte) ([]byte, error) {
	traceIdLength := uint8(len(response.TraceId))
	dataSize := 19 + len(response.Body) + int(traceIdLength)

	buffer = append(buffer, _intToBytes(dataSize-8)...)
	buffer = append(buffer, byte(4))
	buffer = append(buffer, response.Render)
	buffer = append(buffer, traceIdLength)
	buffer = append(buffer, _intToBytes(response.ResultCode)...)
	buffer = append(buffer, _intToBytes(len(response.Body))...)

	buffer = append(buffer, []byte(response.TraceId)...)
	buffer = append(buffer, response.Body...)
	return buffer, nil
}

func UnmarshalProtocolResponse(data []byte) (protocol PlayProtocolResponse, dataSize uint32, err error) {
	if len(data) < 9 {
		return
	}
	if string(data[:4]) != "<<==" {
		return protocol, 0, errors.New("socket protocol head error")
	}

	dataSize = _bytesToUint32(data[4:8]) + 8
	if uint32(len(data)) < dataSize {
		return
	}
	protocol.Version = _bytesToUint8(data[8:9])
	switch protocol.Version {
	case 2:
		err = _unpackResponseV2(data, dataSize, &protocol)
	case 3:
		err = _unpackResponseV3(data, dataSize, &protocol)
	case 4:
		err = _unpackResponseV4(data, dataSize, &protocol)
	default:
		err = errors.New("socket protocol version error")
	}
	return
}

func _unpackResponseV2(buffer []byte, dataSize uint32, protocol *PlayProtocolResponse) (err error) {
	protocol.TraceId = string(buffer[13:45])
	protocol.Body = buffer[45:dataSize]
	return
}

func _unpackResponseV3(buffer []byte, dataSize uint32, protocol *PlayProtocolResponse) (err error) {
	protocol.TagId = _bytesToInt(buffer[9:13])
	protocol.TraceId = string(buffer[13:45])
	protocol.ResultCode = _bytesToInt(buffer[45:49])
	protocol.Body = buffer[49:dataSize]
	return
}

func _unpackResponseV4(buffer []byte, dataSize uint32, protocol *PlayProtocolResponse) (err error) {
	protocol.Render = _bytesToUint8(buffer[9:10])
	traceIdLength := uint8(_bytesToUint8(buffer[10:11]))
	bodyLength := _bytesToUint32(buffer[15:19])

	if uint32(traceIdLength+uint8(bodyLength)+19) > dataSize {
		return errors.New("socket protocol format error")
	}

	idx := uint32(19 + traceIdLength)
	protocol.ResultCode = _bytesToInt(buffer[10:14])
	protocol.TraceId = string(buffer[19:idx])
	protocol.Body = buffer[idx : idx+bodyLength]

	return
}
func _bytesToUint32(data []byte) uint32 {
	var ret uint32
	var l = len(data)
	var i uint = 0
	for i = 0; i < uint(l); i++ {
		ret = ret | (uint32(data[i]) << (i * 8))
	}
	return ret
}

func _bytesToInt(data []byte) int {
	var ret int
	var l = len(data)
	var i uint = 0
	for i = 0; i < uint(l); i++ {
		ret = ret | (int(data[i]) << (i * 8))
	}
	return ret
}

func _bytesToUint8(data []byte) uint8 {
	var ret uint8
	var l = len(data)
	var i uint = 0
	for i = 0; i < uint(l); i++ {
		ret = ret | (uint8(data[i]) << (i * 8))
	}
	return ret
}

func _intToBytes(data int) (ret []byte) {
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

func _uint16ToBytes(data uint16) (ret []byte) {
	var sizeof = unsafe.Sizeof(data)
	ret = make([]byte, sizeof)
	var tmp uint16 = 0xff
	var index uint = 0
	for index = 0; index < uint(sizeof); index++ {
		ret[index] = byte((tmp << (index * 8) & data) >> (index * 8))
	}
	return ret
}

func _nonRespondToByte(data bool) byte {
	if data {
		return 0
	}
	return 1
}

func boolTobyte(data bool) byte {
	if data {
		return 1
	}
	return 0
}
