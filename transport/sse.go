package transport

import (
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/library/golang/json"
	"net/http"
)

type sseTransport struct {
}

func NewSSETransport() *sseTransport {
	return new(sseTransport)
}

func (p *sseTransport) Receive(c *play.Conn) (*play.Request, error) {
	var request play.Request
	request.Respond = true
	request.ActionName, request.Render = ParseHttpPath(c.Http.Request.URL.Path)
	request.InputBinder = ParseHttpInput(c.Http.Request, 1024*4)
	return &request, nil
}

func (p *sseTransport) Send(c *play.Conn, res *play.Response) error {
	var err error
	var data []byte
	var w = c.Http.ResponseWriter

	if data, err = json.Marshal(res.Output.All()); err != nil {
		return err
	}

	if _, err = fmt.Fprintf(w, "data: %s\n\n", string(data)); err != nil {
		return err
	}
	w.(http.Flusher).Flush()
	return err
}
