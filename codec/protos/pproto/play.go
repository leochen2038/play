package pproto

import (
	"context"
	"errors"
	"math"
	"time"
	"unsafe"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/protos/golang/json"
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
// 4 byte : ==>>
// 4 byte : dataSize
// 1 byte : version
// 1 byte : action长度
// 1 byte : respond 0:不需要响应, 1:需要响应
// 1 byte : render 0:json
// 4 byte : header长度
// 4 byte : body长度
// 4 byte : attachment长度
// action header body attachment

// reponse protocol v4
// 4 byte : <<==
// 4 byte : dataSize
// 1 byte : version
// 1 byte : render 0:json
// 4 byte : rc错误码
// 4 byte : header长度
// 4 byte : body长度
// 4 byte : attachment长度
// header body attachment

type requestHeader struct {
	TraceId  string    `key:"traceId" json:"traceId"`
	SpanId   []byte    `key:"spanId" json:"spanId"`
	CallerId int       `key:"callerId" json:"callerId"`
	TagId    int       `key:"tagId" json:"tagId"`
	Deadline time.Time `key:"deadline" json:"deadline"`
}

type responseHeader struct {
	TraceId string `key:"traceId" json:"traceId"`
	TagId   int    `key:"tagId" json:"tagId"`
}
type PlayProtocolRequest struct {
	Version    byte
	Action     string
	NonRespond bool
	Render     byte
	Header     requestHeader
	Body       []byte
	Attachment map[string][]byte
}

type PlayProtocolResponse struct {
	Version    byte
	Render     byte
	ResultCode int
	Header     responseHeader
	Body       []byte
	Attachment map[string][]byte
}

func NewPlayProtocolRequest(ctx context.Context, callerId int, action string, message interface{}) (request PlayProtocolRequest, err error) {
	var body []byte
	if body, err = json.Marshal(message); err != nil {
		return
	}

	request.Header.CallerId = callerId
	if c, ok := ctx.(*play.Context); ok {
		c.Trace.SpanId++
		request.Header.TraceId = c.Trace.TraceId
		request.Header.SpanId = append(c.Trace.ParentSpanId, c.Trace.SpanId)
	} else {
		request.Header.TraceId = play.NewTraceId()
		request.Header.SpanId = []byte{1}
	}
	request.Action = action
	request.Body = body
	return
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
	if len(request.Header.TraceId) > 32 {
		return nil, errors.New("traceId length error")
	}
	if request.Header.CallerId > 0xffff {
		return nil, errors.New("callerId length error")
	}

	buffer = append(buffer, _intToBytes(41+len(request.Body)+len(request.Action))...)
	buffer = append(buffer, byte(2))
	buffer = append(buffer, _nonRespondToByte(request.NonRespond))
	buffer = append(buffer, _uint16ToBytes(uint16(request.Header.CallerId))...)
	buffer = append(buffer, byte(len(request.Action)))
	buffer = append(buffer, _intToBytes(len(request.Body))...)

	buffer = append(buffer, []byte(request.Header.TraceId)...)
	buffer = append(buffer, []byte(request.Action)...)
	buffer = append(buffer, request.Body...)

	return buffer, nil
}

func _packRequestV3(request PlayProtocolRequest, buffer []byte) ([]byte, error) {
	if len(request.Action) > 255 {
		return nil, errors.New("action length error")
	}
	if len(request.Header.TraceId) > 32 {
		return nil, errors.New("traceId length error")
	}
	if len(request.Header.SpanId) > 16 {
		return nil, errors.New("spanId length error")
	}

	buffer = append(buffer, _intToBytes(59+len(request.Action)+len(request.Body))...)
	buffer = append(buffer, byte(3))
	buffer = append(buffer, _intToBytes(request.Header.TagId)...)
	buffer = append(buffer, []byte(request.Header.TraceId)...)

	buffer = append(buffer, request.Header.SpanId...)
	for i := 16 - len(request.Header.SpanId); i > 0; i-- {
		buffer = append(buffer, 0)
	}

	buffer = append(buffer, _intToBytes(request.Header.CallerId)...)
	buffer = append(buffer, byte(len(request.Action)))
	buffer = append(buffer, _nonRespondToByte(request.NonRespond))

	buffer = append(buffer, []byte(request.Action)...)
	buffer = append(buffer, request.Body...)

	return buffer, nil
}

func _packRequestV4(request PlayProtocolRequest, buffer []byte) ([]byte, error) {
	if len(request.Action) > 255 {
		return nil, errors.New("action length error must less than 255")
	}
	if len(request.Header.TraceId) > 255 {
		return nil, errors.New("traceId length error must less than 255")
	}
	if len(request.Header.SpanId) > 255 {
		return nil, errors.New("spanId length error must less than 255")
	}

	header, err := json.Marshal(request.Header)
	if err != nil {
		return nil, err
	}

	var attachmentLen, attachmentCount int = 1, 0
	for k, v := range request.Attachment {
		kl, vl := len(k), len(v)
		if kl > 255 {
			return nil, errors.New("attachment key length error must less than 255")
		}
		if attachmentLen > math.MaxInt32-kl-vl {
			return nil, errors.New("attachment length error must less than 2147483647")
		}
		attachmentCount++
		attachmentLen += len(k) + len(v) + 5
	}

	if attachmentCount > 255 {
		return nil, errors.New("attachment count error must less than 255")
	}

	// dataSize的值不包括==>>4个字节 和 dataSize 4个字节
	var dataSize = 16 + len(request.Action) + len(header) + len(request.Body) + attachmentLen
	if dataSize > math.MaxInt32-8 {
		return nil, errors.New("dataSize length error must less than 2147483647")
	}

	buffer = append(buffer, _intToBytes(dataSize)...)
	buffer = append(buffer, byte(4))

	buffer = append(buffer, uint8(len(request.Action)))
	buffer = append(buffer, boolTobyte(request.NonRespond))
	buffer = append(buffer, request.Render)
	buffer = append(buffer, _intToBytes(len(header))...)
	buffer = append(buffer, _intToBytes(len(request.Body))...)
	buffer = append(buffer, _intToBytes(attachmentLen)...)

	buffer = append(buffer, []byte(request.Action)...)
	buffer = append(buffer, header...)
	buffer = append(buffer, request.Body...)

	buffer = append(buffer, uint8(attachmentCount))
	for k, v := range request.Attachment {
		buffer = append(buffer, uint8(len(k)))
		buffer = append(buffer, []byte(k)...)
		buffer = append(buffer, _intToBytes(len(v))...)
		buffer = append(buffer, v...)
	}

	return buffer, nil
}

// UnpackRequest 解包request请求
func UnmarshalProtocolRequest(data []byte) (protocol PlayProtocolRequest, dataSize uint32, err error) {
	if len(data) < 9 {
		return
	}

	if string(data[:4]) != "==>>" {
		return protocol, 0, errors.New("socket protocol head error:" + string(data[:4]))
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

	protocol.Header.CallerId = _bytesToInt(buffer[10:12])
	protocol.Header.TraceId = string(buffer[17:49])
	protocol.Action = string(buffer[49:actionEndIdx])
	protocol.Body = buffer[actionEndIdx:messageEndIdx]

	return
}

func _unpackRequestV3(buffer []byte, dataSize uint32, protocol *PlayProtocolRequest) (err error) {
	protocol.Header.TagId = _bytesToInt(buffer[9:13])
	protocol.Header.TraceId = string(buffer[13:45])

	for _, v := range buffer[45:61] {
		if v == 0 {
			break
		}
		protocol.Header.SpanId = append(protocol.Header.SpanId, v)
	}

	protocol.Header.CallerId = _bytesToInt(buffer[61:65])
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
	headerLen := _bytesToInt(buffer[12:16])
	bodyLen := _bytesToInt(buffer[16:20])
	attachmentLen := _bytesToInt(buffer[20:24])

	var idx uint32 = 24
	protocol.Action = string(buffer[idx : idx+uint32(actionLength)])
	idx += uint32(actionLength)

	if respond > 0 {
		protocol.NonRespond = true
	}
	if headerLen > 0 {
		if err = json.Unmarshal(buffer[idx:idx+uint32(headerLen)], &protocol.Header); err != nil {
			return
		}
		idx += uint32(headerLen)
	}
	if bodyLen > 0 {
		protocol.Body = buffer[idx : idx+uint32(bodyLen)]
		idx += uint32(bodyLen)
	}

	if attachmentLen > 0 {
		attachmentCount := _bytesToUint8(buffer[idx : idx+1])
		idx += 1
		protocol.Attachment = make(map[string][]byte)
		for i := uint8(0); i < attachmentCount; i++ {
			keyLen := _bytesToUint8(buffer[idx : idx+1])
			idx += 1
			key := string(buffer[idx : idx+uint32(keyLen)])
			idx += uint32(keyLen)
			valueLen := _bytesToInt(buffer[idx : idx+4])
			idx += 4
			protocol.Attachment[key] = buffer[idx : idx+uint32(valueLen)]
			idx += uint32(valueLen)
		}
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
	buffer = append(buffer, []byte(response.Header.TraceId)...)
	buffer = append(buffer, response.Body...)

	return buffer, nil
}

func _packResponseV3(response PlayProtocolResponse, buffer []byte) ([]byte, error) {
	protocolSize := 49 + len(response.Body)

	buffer = append(buffer, _intToBytes(protocolSize-8)...)
	buffer = append(buffer, byte(3))
	buffer = append(buffer, _intToBytes(response.Header.TagId)...)
	buffer = append(buffer, []byte(response.Header.TraceId)...)

	buffer = append(buffer, _intToBytes(response.ResultCode)...)
	buffer = append(buffer, response.Body...)

	return buffer, nil
}

func _packResponseV4(response PlayProtocolResponse, buffer []byte) ([]byte, error) {
	header, err := json.Marshal(response.Header)
	if err != nil {
		return nil, err
	}

	var attachmentLen, attachmentCount int = 1, 0
	for k, v := range response.Attachment {
		kl, vl := len(k), len(v)
		if kl > 255 {
			return nil, errors.New("attachment key length error must less than 255")
		}
		if attachmentLen > math.MaxInt32-kl-vl {
			return nil, errors.New("attachment length error must less than 2147483647")
		}
		attachmentCount++
		attachmentLen += len(k) + len(v) + 5
	}

	if attachmentCount > 255 {
		return nil, errors.New("attachment count error must less than 255")
	}

	// dataSize的值不包括==>>4个字节 和 dataSize 4个字节
	dataSize := 18 + len(header) + len(response.Body) + attachmentLen

	buffer = append(buffer, _intToBytes(dataSize)...)
	buffer = append(buffer, byte(4))
	buffer = append(buffer, response.Render)
	buffer = append(buffer, _intToBytes(response.ResultCode)...)
	buffer = append(buffer, _intToBytes(len(header))...)
	buffer = append(buffer, _intToBytes(len(response.Body))...)
	buffer = append(buffer, _intToBytes(attachmentLen)...)

	buffer = append(buffer, header...)
	buffer = append(buffer, response.Body...)

	buffer = append(buffer, byte(attachmentCount))
	for k, v := range response.Attachment {
		buffer = append(buffer, byte(len(k)))
		buffer = append(buffer, []byte(k)...)
		buffer = append(buffer, _intToBytes(len(v))...)
		buffer = append(buffer, v...)
	}
	return buffer, nil
}

func UnmarshalProtocolResponse(data []byte) (protocol PlayProtocolResponse, dataSize uint32, err error) {
	if len(data) < 9 {
		return
	}
	if string(data[:4]) != "<<==" && string(data[:4]) != "==>>" {
		return protocol, 0, errors.New("socket protocol head error:" + string(data[:4]) + ". data is:" + string(data))
	}

	size := _bytesToUint32(data[4:8]) + 8
	if uint32(len(data)) < size {
		return
	}
	dataSize = size
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
	protocol.Header.TraceId = string(buffer[13:45])
	protocol.Body = buffer[45:dataSize]
	return
}

func _unpackResponseV3(buffer []byte, dataSize uint32, protocol *PlayProtocolResponse) (err error) {
	protocol.Header.TagId = _bytesToInt(buffer[9:13])
	protocol.Header.TraceId = string(buffer[13:45])
	protocol.ResultCode = _bytesToInt(buffer[45:49])
	protocol.Body = buffer[49:dataSize]
	return
}

func _unpackResponseV4(buffer []byte, dataSize uint32, protocol *PlayProtocolResponse) (err error) {
	protocol.Render = _bytesToUint8(buffer[9:10])
	protocol.ResultCode = _bytesToInt(buffer[10:14])
	headerLen := _bytesToInt(buffer[14:18])
	bodyLen := _bytesToInt(buffer[18:22])
	attachmentLen := _bytesToInt(buffer[22:26])

	var idx uint32 = 26
	if headerLen > 0 {
		err = json.Unmarshal(buffer[idx:idx+uint32(headerLen)], &protocol.Header)
		if err != nil {
			return
		}
		idx += uint32(headerLen)
	}

	if bodyLen > 0 {
		protocol.Body = buffer[idx : idx+uint32(bodyLen)]
		idx += uint32(bodyLen)
	}

	if attachmentLen > 0 {
		attachmentCount := _bytesToUint8(buffer[idx : idx+1])
		idx++
		protocol.Attachment = make(map[string][]byte, attachmentCount)
		for i := uint8(0); i < attachmentCount; i++ {
			keyLen := _bytesToUint8(buffer[idx : idx+1])
			idx += 1
			key := string(buffer[idx : idx+uint32(keyLen)])
			idx += uint32(keyLen)
			valueLen := _bytesToInt(buffer[idx : idx+4])
			idx += 4
			protocol.Attachment[key] = buffer[idx : idx+uint32(valueLen)]
			idx += uint32(valueLen)
		}
	}

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
