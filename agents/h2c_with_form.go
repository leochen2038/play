package agents

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"reflect"
	"strings"
	"time"

	"golang.org/x/net/http2"

	"github.com/leochen2038/play/codec/protos/golang/json"
)

var H2cWithForm = &h2cWithForm{router: make(map[string]string)}

type h2cWithForm struct {
	router map[string]string
}

func (a *h2cWithForm) SetRouter(servie string, host string) {
	if !strings.Contains(host, "http") {
		host = "http://" + host
	}
	a.router[servie] = host
}

func (a *h2cWithForm) Request(ctx context.Context, service string, action string, body []byte) ([]byte, error) {
	var err error
	var host string
	var resp *http.Response
	if host = a.router[service]; host == "" {
		return nil, errors.New("service:" + service + " router not found")
	}

	url := host + "/" + strings.ReplaceAll(action, ".", "/")

	req, err := http.NewRequestWithContext(context.Background(), "post", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "multipart/form-data; boundary="+getBoundary())
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

func (a *h2cWithForm) Marshal(ctx context.Context, service string, action string, i interface{}) ([]byte, error) {
	bodyBuff := bytes.NewBufferString("")
	bodyWriter := multipart.NewWriter(bodyBuff)
	_ = bodyWriter.SetBoundary(getBoundary())

	v, t := reflect.ValueOf(i), reflect.TypeOf(i)
	for index := 0; index < v.NumField(); index++ {
		name := t.Field(index).Tag.Get("json")
		if name == "" {
			name = t.Field(index).Name
		}

		fieldType := t.Field(index).Type.String()
		if fieldType != "[]string" && fieldType != "string" && fieldType != "[]uint8" {
			return nil, errors.New("field " + name + " type not in string,[]string,[]uint8")
		}

		// 写普通字段
		if fieldType == "[]string" {
			for _, value := range v.Field(index).Interface().([]string) {
				_ = bodyWriter.WriteField(name, value)
			}
		} else if fieldType == "string" {
			_ = bodyWriter.WriteField(name, v.Field(index).Interface().(string))
		}
	}

	for index := 0; index < v.NumField(); index++ {
		name := t.Field(index).Tag.Get("json")
		if name == "" {
			name = t.Field(index).Name
		}

		// 写文件内容
		fieldType := t.Field(index).Type.String()
		if fieldType == "[]uint8" {
			fileWriter, err := bodyWriter.CreateFormFile(name, name)
			if err != nil {
				return nil, err
			}
			_, err = fileWriter.Write(v.Field(index).Interface().([]byte))
			if err != nil {
				return nil, err
			}
		}
	}
	_ = bodyWriter.Close()

	return bodyBuff.Bytes(), nil
}

func (a *h2cWithForm) Unmarshal(ctx context.Context, service string, action string, data []byte, i interface{}) error {
	return json.Unmarshal(data, i)
}

var _client *http.Client

func getClient() *http.Client {
	if _client != nil {
		return _client
	}

	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}

	_client = &http.Client{Transport: transport, Timeout: 30 * time.Second}
	return _client
}

var _boundary string

func getBoundary() string {
	if _boundary != "" {
		return _boundary
	}

	var buf [30]byte
	_, err := io.ReadFull(rand.Reader, buf[:])
	if err != nil {
		panic(err)
	}
	_boundary = fmt.Sprintf("%x", buf[:])
	return _boundary
}
