package agents

import (
	"context"
	"crypto/tls"
	"io"
	"sync"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/protos/golang/json"
	"github.com/leochen2038/play/codec/protos/pproto"
	"github.com/leochen2038/play/config"
	"github.com/lucas-clemente/quic-go"
)

var quicConnetions = sync.Map{}

type quicPProtoAgent struct {
	addr       string
	nextProtos []string
	connection quic.Connection
	config     *quic.Config
}

func GetQuicPProtoAgent(host string, nextProtos []string, config *quic.Config) (agent *quicPProtoAgent, err error) {
	if agent, ok := quicConnetions.Load(host); ok {
		return agent.(*quicPProtoAgent), nil
	}
	if len(nextProtos) == 0 {
		nextProtos = []string{"quicServer"}
	}
	connection, err := content(host, nextProtos, config)
	if err != nil {
		return nil, err
	}
	agent = &quicPProtoAgent{
		addr:       host,
		nextProtos: nextProtos,
		config:     config,
		connection: connection,
	}
	quicConnetions.Store(host, agent)
	return agent, nil
}

func (q *quicPProtoAgent) getStream() (quic.Stream, error) {
	var err error
	var stream quic.Stream
	if stream, err = q.connection.OpenStreamSync(context.Background()); err != nil {
		if connection, err := content(q.addr, q.nextProtos, q.config); err != nil {
			return nil, err
		} else {
			q.connection = connection
			return q.connection.OpenStreamSync(context.Background())
		}
	}
	return stream, err
}

func content(addr string, nextprotos []string, config *quic.Config) (quic.Connection, error) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         nextprotos,
	}
	return quic.DialAddr(addr, tlsConf, config)
}

func (a *quicPProtoAgent) Request(ctx context.Context, service string, action string, body []byte) ([]byte, error) {
	var err error
	var stream quic.Stream

	if stream, err = a.getStream(); err != nil {
		return nil, err
	}
	defer stream.Close()

	if _, err = stream.Write(body); err != nil {
		return nil, err
	}
	var heaer = make([]byte, 8)
	if _, err = io.ReadFull(stream, heaer); err != nil {
		return nil, err
	}

	dataSize := _bytesToUint32(heaer[4:8])
	buffer := make([]byte, dataSize+8)
	copy(buffer, heaer)
	if _, err = io.ReadFull(stream, buffer[8:]); err != nil {
		return nil, err
	}

	return buffer, nil
}

func (a *quicPProtoAgent) Marshal(ctx context.Context, service string, action string, i interface{}) ([]byte, error) {
	var err error
	var body []byte

	if body, err = json.Marshal(i); err != nil {
		return nil, err
	}

	request := pproto.PlayProtocolRequest{Action: action, Body: body}
	request.Header.CallerId, _ = config.Int("appid")
	if c, ok := ctx.(*play.Context); ok {
		c.Trace.SpanId++
		request.Header.TraceId = c.Trace.TraceId
		request.Header.SpanId = append(c.Trace.ParentSpanId, c.Trace.SpanId)
	}
	return pproto.MarshalProtocolRequest(request)
}

func (a *quicPProtoAgent) Unmarshal(ctx context.Context, service string, action string, data []byte, i interface{}) error {
	if response, _, err := pproto.UnmarshalProtocolResponse(data); err != nil {
		return err
	} else {
		return json.Unmarshal(response.Body, i)
	}
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
