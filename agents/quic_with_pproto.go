package agents

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"sync"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/protos/golang/json"
	"github.com/leochen2038/play/codec/protos/pproto"
	"github.com/leochen2038/play/config"
	"github.com/lucas-clemente/quic-go"
)

var QuicWithPProto = &quicWithPProto{}

type quicClient struct {
	addr       string
	nextProtos []string
	instance   quic.Connection
	config     *quic.Config
}

func (q *quicClient) getStream() (quic.Stream, error) {
	var err error
	var stream quic.Stream
	if stream, err = q.instance.OpenStreamSync(context.Background()); err != nil {
		if q.instance, err = content(q.addr, q.nextProtos, q.config); err != nil {
			return nil, err
		}
		stream, err = q.instance.OpenStreamSync(context.Background())
	}
	return stream, err
}

type quicWithPProto struct {
	router sync.Map
}

func (a *quicWithPProto) SetRouter(servie string, url string, nextProtos []string, config *quic.Config) (err error) {
	if len(nextProtos) == 0 {
		nextProtos = []string{"quicServer"}
	}
	quicClient := &quicClient{
		addr:       url,
		nextProtos: nextProtos,
		config:     config,
	}
	if quicClient.instance, err = content(url, nextProtos, config); err != nil {
		return err
	}
	a.router.Store(servie, quicClient)
	return nil
}

func content(addr string, nextprotos []string, config *quic.Config) (quic.Connection, error) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         nextprotos,
	}
	return quic.DialAddr(addr, tlsConf, config)
}

func (a *quicWithPProto) Request(ctx context.Context, service string, action string, body []byte) ([]byte, error) {
	var ok bool
	var err error
	var r interface{}
	var q *quicClient
	var stream quic.Stream

	if r, ok = a.router.Load(service); !ok {
		return nil, errors.New("service: " + service + " router not found")
	}
	if q, ok = r.(*quicClient); !ok {
		return nil, errors.New("service: " + service + " not quicClient")
	}

	if stream, err = q.getStream(); err != nil {
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

func (a *quicWithPProto) Marshal(ctx context.Context, service string, action string, i interface{}) ([]byte, error) {
	var err error
	var body []byte
	caller, _ := config.Int("appid")

	if body, err = json.Marshal(i); err != nil {
		return nil, err
	}
	if c, ok := ctx.(*play.Context); ok {
		c.Trace.SpanId++
		return pproto.MarshalProtocolRequest(pproto.PlayProtocolRequest{
			Action:   action,
			Body:     body,
			TraceId:  c.Trace.TraceId,
			SpanId:   append(c.Trace.ParentSpanId, c.Trace.SpanId),
			CallerId: caller,
		})
	} else {
		return pproto.MarshalProtocolRequest(pproto.PlayProtocolRequest{
			Action:   action,
			Body:     body,
			CallerId: caller,
		})
	}
}

func (a *quicWithPProto) Unmarshal(ctx context.Context, service string, action string, data []byte, i interface{}) error {
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
