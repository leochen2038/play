package agents

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
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

	myForm := new(form)
	err = json.Unmarshal(body, &myForm)
	if err != nil {
		return nil, err
	}

	//fmt.Println("Request myform", myForm)

	bodyBuf := bytes.NewBufferString("")
	bodyWriter := multipart.NewWriter(bodyBuf)

	for _, data := range myForm.Fields {
		if reflect.TypeOf(data.Value).Kind() == reflect.Slice {
			for _, val := range data.Value.([]interface{}) {
				_ = bodyWriter.WriteField(data.Name, val.(string))
			}
		} else {
			_ = bodyWriter.WriteField(data.Name, data.Value.(string))
		}
	}

	for _, data := range myForm.Files {
		fileWriter, err := bodyWriter.CreateFormFile(data.FileName, data.FileName)
		if err != nil {
			return nil, err
		}
		_, err = fileWriter.Write(data.FileData)
		if err != nil {
			return nil, err
		}
	}

	_ = bodyWriter.Close()

	req, err := http.NewRequestWithContext(context.Background(), "post", url, bodyBuf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "multipart/form-data; boundary="+bodyWriter.Boundary())
	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}

	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}
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
	var err error
	var body []byte

	myForm := new(form)
	v := reflect.ValueOf(i)
	t := reflect.TypeOf(i)
	for i := 0; i < v.NumField(); i++ {
		name := t.Field(i).Tag.Get("json")
		if name == "" {
			name = t.Field(i).Name
		}

		fieldType := t.Field(i).Type.String()
		if fieldType != "string" && fieldType != "[]string" && fieldType != "[]uint8" {
			return body, errors.New("field " + name + " type not in string,[]string,[]uint8")
		}

		if fieldType == "[]uint8" {
			myForm.Files = append(myForm.Files, file{
				FileName: name,
				FileData: v.Field(i).Bytes(),
			})
		} else if fieldType == "[]string" {
			myForm.Fields = append(myForm.Fields, field{
				Name:  name,
				Value: v.Field(i).Interface().([]string),
			})
		} else {
			myForm.Fields = append(myForm.Fields, field{
				Name:  name,
				Value: v.Field(i).Interface().(string),
			})
		}
	}

	//fmt.Printf("Marshal myform %#v \n", myForm)

	if body, err = json.Marshal(myForm); err != nil {
		return nil, err
	}
	return body, nil
}

func (a *h2cWithForm) Unmarshal(ctx context.Context, service string, action string, data []byte, i interface{}) error {
	return json.Unmarshal(data, i)
}

type form struct {
	Files  []file
	Fields []field
}

type file struct {
	FileName string
	FileData []byte
}

type field struct {
	Name  string
	Value interface{}
}
