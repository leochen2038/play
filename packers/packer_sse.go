package packers

import (
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/middleware/golang/json"
	"net/http"
)

type SSEPacker struct  {
}

func (p *SSEPacker)Read(c *play.Conn, data []byte) (*play.Request, []byte, error) {
	var request play.Request
	request.Respond = true
	request.ActionName, request.Render = ParseHttpPath(c.Http.Request.URL.Path)
	request.Parser = ParseHttpInput(c.Http.Request, 1024*4)
	return &request, nil, nil
}

func (p *SSEPacker)Write(c *play.Conn, output play.Output) error {
	var err error
	var data []byte
	var w = c.Http.Response

	if data, err = json.Marshal(output.All()); err != nil {
		return err
	}

	if _, err = fmt.Fprintf(w, "data: %s\n\n", string(data)); err != nil {
		return  err
	}
	w.(http.Flusher).Flush()
	return  err
}