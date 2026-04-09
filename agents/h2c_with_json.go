package agents

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/leochen2038/play/codec/protos/golang/json"
)

var H2cWithJson = &h2cWithJson{router: make(map[string]string)}

type h2cWithJson struct {
	router map[string]string
}

func (a *h2cWithJson) SetRouter(servie string, host string) {
	if !strings.Contains(host, "http") {
		host = "http://" + host
	}
	a.router[servie] = host
}

func (a *h2cWithJson) Request(ctx context.Context, service string, action string, body []byte) ([]byte, error) {
	var err error
	var host string
	var resp *http.Response
	if host = a.router[service]; host == "" {
		return nil, errors.New("service:" + service + " router not found")
	}

	url := host + "/" + strings.ReplaceAll(action, ".", "/")

	req, err := http.NewRequestWithContext(ctx, "post", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := getClient()
	if resp, err = client.Do(req); err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("http status error:" + resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func (a *h2cWithJson) Marshal(ctx context.Context, service string, action string, i interface{}) ([]byte, error) {
	var err error
	var body []byte

	if body, err = json.Marshal(i); err != nil {
		return nil, err
	}
	return body, nil
}

func (a *h2cWithJson) Unmarshal(ctx context.Context, service string, action string, data []byte, i interface{}) error {
	return json.Unmarshal(data, i)
}
