package packers

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/binders"
	"github.com/leochen2038/play/codec/renders"
)

type HttpPacker struct {
}

func NewHttpPackert() play.IPacker {
	return new(HttpPacker)
}

func (p *HttpPacker) Receive(c *play.Conn) (*play.Request, error) {
	var request = new(play.Request)
	request.ActionName, request.RenderName = ParseHttpPath(c.Http.Request.URL.Path)
	request.InputBinder = ParseHttpInput(c.Http.Request)
	return request, nil
}

func (p *HttpPacker) Pack(c *play.Conn, res *play.Response) (data []byte, err error) {
	switch res.RenderName {
	case "json":
		c.Http.ResponseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
		c.Http.ResponseWriter.Header().Set("Cache-Control", "no-cache, must-revalidate, max-age=0")
		return renders.GetRenderOfJson().Render(res.Output.All())
	default:
		return nil, errors.New("undefined " + res.RenderName + " http response render")
	}
}

func ParseHttpPath(path string) (action string, render string) {
	if indexDot := strings.Index(path, "."); indexDot > 0 {
		render = path[indexDot+1:]
		path = path[:indexDot]
	}
	if path == "/" || path == "" {
		path = "/index"
	}
	if render == "" {
		render = "json"
	}

	action = strings.ReplaceAll(path[1:], "/", ".")

	return
}

func ParseHttpInput(request *http.Request) binders.Binder {
	contentType := request.Header.Get("Content-Type")

	if strings.Contains(contentType, "/json") {
		raw, _ := ioutil.ReadAll(request.Body)
		_ = request.Body.Close()
		request.Body = ioutil.NopCloser(bytes.NewBuffer(raw))
		return binders.GetBinderOfJson(raw)
	}

	if strings.Contains(contentType, "/bytes") {
		raw, _ := ioutil.ReadAll(request.Body)
		_ = request.Body.Close()
		request.Body = ioutil.NopCloser(bytes.NewBuffer(raw))
		return binders.GetBinderOfBytes(raw)
	}

	if strings.Contains(contentType, "/x-www-form-urlencoded") {
		_ = request.ParseForm()
		return binders.GetBinderOfUrlValue(request.Form, nil)
	}

	if strings.Contains(contentType, "/form-data") {
		_ = request.ParseMultipartForm(4096)
		return binders.GetBinderOfUrlValue(request.Form, request.MultipartForm.File)
	}
	return binders.GetBinderOfUrlValue(request.URL.Query(), nil)
}
