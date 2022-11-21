package agents

import (
	"context"
	"log"
	"net"

	"github.com/leochen2038/play/codec/protos/golang/json"
	"github.com/leochen2038/play/codec/protos/pproto"
)

type PlaySocket struct {
	routerHandle func(ctx context.Context, service, action string) string
}

func (a *PlaySocket) SetRouterHandle(handle func(ctx context.Context, service, action string) string) {
	a.routerHandle = handle
}

func (a *PlaySocket) Request(ctx context.Context, service string, action string, body []byte) ([]byte, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", a.routerHandle(ctx, service, action))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if _, err := conn.Write(body); err != nil {
		return nil, err
	}
	var buffer = make([]byte, 4096)
	var surplus []byte
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			log.Println("[play server]", err, "on", conn.RemoteAddr().String())
			return nil, err
		}
		protocol, dataSize, err := pproto.UnmarshalProtocolResponse(append(surplus, buffer[:n]...))
		if err != nil {
			log.Println("[play server]", err, "on", conn.RemoteAddr().String())
			return nil, err
		}
		if dataSize > 0 {
			return protocol.Body, nil
		}
	}
}

func (a *PlaySocket) Marshal(ctx context.Context, service string, action string, i interface{}) ([]byte, error) {
	var err error
	var body []byte

	if body, err = json.Marshal(i); err != nil {
		return nil, err
	}
	return pproto.MarshalProtocolRequest(pproto.PlayProtocolRequest{
		Action: action,
		Body:   body,
	})
}

func (a *PlaySocket) Unmarshal(ctx context.Context, service string, action string, data []byte, i interface{}) error {
	return json.Unmarshal(data, i)
}
