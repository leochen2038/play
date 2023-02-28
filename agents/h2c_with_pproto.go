package agents

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/leochen2038/play/codec/protos/pproto"
	"golang.org/x/net/http2"

	"github.com/leochen2038/play/codec/protos/golang/json"
)

var h2cPProtoRoute sync.Map
var h2cClient *http.Client

type h2cPProtoAgent struct {
	host   string
	config map[string]interface{}
}

func SetH2cPProtoRouter(name string, host string, config map[string]interface{}) {
	h2cPProtoRoute.Store(name, &h2cPProtoAgent{host: host, config: config})
}

func GetH2cPProtoAgent(name string) (*h2cPProtoAgent, error) {
	if agent, ok := h2cPProtoRoute.Load(name); !ok {
		return nil, errors.New("not found agent by:" + name)
	} else {
		return agent.(*h2cPProtoAgent), nil
	}
}

func getH2cClient(config map[string]interface{}) *http.Client {
	if h2cClient != nil {
		return h2cClient
	}
	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}

	var timeout = 500 * time.Millisecond
	if t, ok := config["timeout"]; ok {
		timeout = t.(time.Duration)
	}
	h2cClient = &http.Client{Transport: transport, Timeout: timeout}

	return h2cClient
}

func (a *h2cPProtoAgent) Request(ctx context.Context, service string, action string, body []byte) ([]byte, error) {
	var err error
	var resp *http.Response

	url := a.host + "/" + strings.ReplaceAll(action, ".", "/")

	req, err := http.NewRequestWithContext(ctx, "post", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	client := getH2cClient(a.config)
	if resp, err = client.Do(req); err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("http status error:" + resp.Status)
	}

	return ioutil.ReadAll(resp.Body)
}

func (a *h2cPProtoAgent) Marshal(ctx context.Context, service string, action string, i interface{}) ([]byte, error) {
	request, err := pproto.NewPlayProtocolRequest(ctx, callerId, action, i)
	if err != nil {
		return nil, err
	}
	return pproto.MarshalProtocolRequest(request)
}

func (a *h2cPProtoAgent) Unmarshal(ctx context.Context, service string, action string, data []byte, i interface{}) error {
	if response, _, err := pproto.UnmarshalProtocolResponse(data); err != nil {
		return err
	} else {
		return json.Unmarshal(response.Body, i)
	}
}
