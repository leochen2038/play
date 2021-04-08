package packers

import (
	"bytes"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/middleware/golang/json"
	"github.com/leochen2038/play/parsers"
	"io/ioutil"
	"net/http"
	"strings"
)

type HttpPacker struct  {
	InputMaxSize int64
	DefaultRender string
}

func (p *HttpPacker)Read(c *play.Client, data []byte) (*play.Request, []byte, error) {
	var request play.Request

	request.Respond = true
	request.ActionName, request.Render = ParseHttpPath(c.Http.Request.URL.Path)
	request.Parser = ParseHttpInput(c.Http.Request, p.InputMaxSize)
	return &request, nil, nil
}

func (p *HttpPacker)Write(c *play.Client, output play.Output)  error {
	var err error
	var data []byte
	var render = c.Http.Render
	if render == "" {
		render = p.DefaultRender
	}

	if data, err = json.Marshal(output.All()); err != nil {
		return err
	}
	_, err = c.Http.Response.Write(data)
	return err
}

func ParseHttpPath(path string) (action string, render string) {
	if indexDot := strings.Index(path, "."); indexDot > 0 {
		action = path[:indexDot]
		render = path[indexDot+1:]
	}
	if path == "/" || path == "" {
		path = "/index"
	}
	action = strings.ReplaceAll(path[1:], "/", ".")
	return
}

func ParseHttpInput(request *http.Request, formMaxMemory int64) parsers.Parser {
	contentType := request.Header.Get("Content-Type")

	if strings.Contains(contentType, "/json") {
		raw, _ := ioutil.ReadAll(request.Body)
		_ = request.Body.Close()
		request.Body = ioutil.NopCloser(bytes.NewBuffer(raw))
		return parsers.NewJsonParser(raw)
	}

	if strings.Contains(contentType, "/x-www-form-urlencoded") {
		_ = request.ParseForm()
		return parsers.NewFormDataParser(request.Form)
	}

	if strings.Contains(contentType, "/form-data") {
		_ = request.ParseMultipartForm(formMaxMemory)
		return parsers.NewFormDataParser(request.Form)
	}

	return parsers.NewFormDataParser(request.URL.Query())
}