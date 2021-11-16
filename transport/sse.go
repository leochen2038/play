package transport

import (
	"errors"
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/library/golang/json"
	"net/http"
	"runtime/debug"
)

type SseTransport struct {
}

func NewSSETransport() *SseTransport {
	return new(SseTransport)
}

func (p *SseTransport) Receive(c *play.Conn) (*play.Request, error) {
	var request play.Request
	request.Respond = true
	request.ActionName, request.Render = ParseHttpPath(c.Http.Request.URL.Path)
	request.InputBinder = ParseHttpInput(c.Http.Request, 1024*4)
	request.Render = "json"
	return &request, nil
}

func (p *SseTransport) Response(c *play.Conn, res *play.Response) error {
	var err error
	var data []byte
	var w = c.Http.ResponseWriter
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack()))
		}
	}()

	if res.Render != "json" {
		return errors.New("undefined " + res.Render + " sse response render")
	}
	if data, err = json.MarshalEscape(res.Output.All(), false, false); err != nil {
		return err
	}

	if _, err = fmt.Fprintf(w, "data: %s\n\n", string(data)); err != nil {
		return err
	}
	w.(http.Flusher).Flush()
	return err
}
