package packers

import (
	"errors"
	"io"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/binders"
	"github.com/leochen2038/play/codec/protos/pproto"
	"github.com/leochen2038/play/codec/renders"
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
// 1  byte : render
// 1  byte : traceId长度
// 4  byte : rc错误码
// 4  byte : body长度

type PlayPacker struct {
}

func NewPlayPacker() play.IPacker {
	return new(PlayPacker)
}

func (p *PlayPacker) Receive(c *play.Conn) (*play.Request, error) {
	var err error
	var dataSize uint32
	var protocol pproto.PlayProtocolRequest
	var buffer []byte

	if c.Type == play.SERVER_TYPE_TCP {
		buffer = c.Tcp.Surplus
	} else if c.Type == play.SERVER_TYPE_QUIC {
		var heaer = make([]byte, 8)
		if _, err = io.ReadFull(c.Quic.Stream, heaer); err != nil {
			return nil, err
		}
		dataSize = _bytesToUint32(heaer[4:8])
		buffer = make([]byte, dataSize+8)
		copy(buffer, heaer)
		if _, err = io.ReadFull(c.Quic.Stream, buffer[8:]); err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("unknown conn type")
	}
	if protocol, dataSize, err = pproto.UnmarshalProtocolRequest(buffer); err != nil {
		return nil, err
	}

	if dataSize > 0 {
		if c.Type == play.SERVER_TYPE_TCP {
			c.Tcp.Surplus = buffer[dataSize:]
		}
		return &play.Request{
			Version:     protocol.Version,
			ActionName:  protocol.Action,
			TraceId:     protocol.TraceId,
			SpanId:      protocol.SpanId,
			CallerId:    protocol.CallerId,
			TagId:       protocol.TagId,
			NonRespond:  protocol.NonRespond,
			InputBinder: binders.GetBinderOfJson(protocol.Body),
		}, nil
	}
	return nil, nil
}

func (p *PlayPacker) Pack(c *play.Conn, res *play.Response) (data []byte, err error) {
	var rc int
	var body []byte
	var buffer []byte

	if res.Error != nil {
		if res, ok := res.Error.(play.Err); ok {
			rc = res.Code()
		} else {
			rc = 0x1
		}
	}
	if len(res.Output.All()) > 0 {
		if body, _ = renders.GetRenderOfJson().Render(res.Output.All()); err != nil {
			return nil, err
		}
	}

	var version = res.Version
	if version == 0 {
		if c.Type == play.SERVER_TYPE_TCP {
			version = c.Tcp.Version
		} else if c.Type == play.SERVER_TYPE_QUIC {
			version = c.Quic.Version
		}
	}
	if buffer, err = pproto.MarshalProtocolResponse(pproto.PlayProtocolResponse{
		Version:    version,
		TraceId:    res.TraceId,
		ResultCode: rc,
		Body:       body,
	}); err != nil {
		return nil, err
	}

	return buffer, nil
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
