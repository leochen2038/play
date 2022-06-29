package transport

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/binder"
)

type HttpTransport struct {
	inputMaxSize  int64
	defaultRender string
}

func NewHttpTransport(inputMaxSize int64) *HttpTransport {
	return &HttpTransport{inputMaxSize: inputMaxSize, defaultRender: "json"}
}

func (p *HttpTransport) Receive(c *play.Conn) (*play.Request, error) {
	var request = new(play.Request)

	request.Respond = true
	request.ActionName, request.Render = ParseHttpPath(c.Http.Request.URL.Path)
	request.InputBinder = ParseHttpInput(c.Http.Request, p.inputMaxSize)

	if request.Render == "" {
		request.Render = p.defaultRender
	}
	return request, nil
}

func (p *HttpTransport) Send(c *play.Conn, res *play.Response) (err error) {

	switch res.Render {
	case "json":
		err = HttpSendJson(c.Http.ResponseWriter, res.Output)
	default:
		err = errors.New("undefined " + res.Render + " http response render")
	}

	return err
}

func HttpSendJson(w http.ResponseWriter, output play.Output) error {
	var err error
	var data []byte

	if data, err = output.ToJsonRaw(); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, must-revalidate, max-age=0")
	_, err = w.Write(data)

	return err
}

func ParseHttpPath(path string) (action string, render string) {
	if indexDot := strings.Index(path, "."); indexDot > 0 {
		render = path[indexDot+1:]
		path = path[:indexDot]
	}
	if path == "/" || path == "" {
		path = "/index"
	}

	action = strings.ReplaceAll(path[1:], "/", ".")

	return
}

func ParseHttpInput(request *http.Request, formMaxMemory int64) play.Binder {
	contentType := request.Header.Get("Content-Type")

	if strings.Contains(contentType, "/json") {
		raw, _ := ioutil.ReadAll(request.Body)
		_ = request.Body.Close()
		request.Body = ioutil.NopCloser(bytes.NewBuffer(raw))
		return binder.NewJsonBinder(raw)
	}
	if strings.Contains(contentType, "/bytes") {
		raw, _ := ioutil.ReadAll(request.Body)
		_ = request.Body.Close()
		request.Body = ioutil.NopCloser(bytes.NewBuffer(raw))
		return binder.NewBytesBinder(raw)
	}

	if strings.Contains(contentType, "/x-www-form-urlencoded") {
		_ = request.ParseForm()
		return binder.NewUrlValueBinder(request.Form)
	}

	if strings.Contains(contentType, "/form-data") {
		_ = request.ParseMultipartForm(formMaxMemory)
		return binder.NewUrlValueBinder(request.Form)
	}
	return binder.NewUrlValueBinder(request.URL.Query())
}
