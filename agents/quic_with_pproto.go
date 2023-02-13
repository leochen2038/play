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
	"github.com/lucas-clemente/quic-go"
)

var callerId int = 0
var quicRouter = sync.Map{}

type quicPProtoAgent struct {
	addr       string
	nextProtos []string
	connection quic.Connection
	config     *quic.Config
}

func SetCallerId(id int) {
	callerId = id
}

func SetQuicRouter(name string, host string, nextProtos []string, config *quic.Config) {
	if len(nextProtos) == 0 {
		nextProtos = []string{"quicServer"}
	}
	quicRouter.Store(name, &quicPProtoAgent{
		addr:       host,
		nextProtos: nextProtos,
		config:     config,
		connection: nil,
	})
}

func GetQuicPProtoAgent(name string) (agent *quicPProtoAgent, err error) {
	if i, ok := quicRouter.Load(name); !ok {
		return nil, errors.New("not found agent by:" + name)
	} else {
		agent = i.(*quicPProtoAgent)
		if agent.connection == nil {
			agent.connection, err = content(agent.addr, agent.nextProtos, agent.config)
		}
	}

	return agent, err
}

func (q *quicPProtoAgent) getStream(ctx context.Context) (stream quic.Stream, err error) {
	if q.connection == nil {
		if q.connection, err = content(q.addr, q.nextProtos, q.config); err != nil {
			return nil, errors.New("connect to " + q.addr + " error:" + err.Error())
		}
	}
	if stream, err = q.connection.OpenStreamSync(ctx); err != nil {
		if q.connection, err = content(q.addr, q.nextProtos, q.config); err != nil {
			return nil, errors.New("connect after open stream err " + q.addr + " error:" + err.Error())
		}
		stream, err = q.connection.OpenStreamSync(ctx)
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

	if stream, err = a.getStream(ctx); err != nil {
		return nil, err
	}
	defer stream.Close()
	if deadline, ok := ctx.Deadline(); ok {
		stream.SetDeadline(deadline)
	}
	if _, err = stream.Write(body); err != nil {
		return nil, errors.New("write to " + a.addr + " error:" + err.Error())
	}
	var heaer = make([]byte, 8)
	if _, err = io.ReadFull(stream, heaer); err != nil {
		var traceId string
		if c, ok := ctx.(*play.Context); ok {
			traceId = c.Trace.TraceId
		}
		return nil, errors.New("traceId:" + traceId + ". read header from " + a.addr + " error:" + err.Error())
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
	request, err := pproto.NewPlayProtocolRequest(ctx, callerId, action, i)
	if err != nil {
		return nil, err
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
