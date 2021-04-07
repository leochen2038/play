package packers

import (
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/middleware/golang/json"
	"net/http"
)

type SSEPacker struct  {
	InputMaxSize int64
}

func (p *SSEPacker)Read(c *play.Client, data []byte) (*play.Request, []byte, error) {
	var request play.Request
	request.Respond = true
	request.ActionName, request.Render = ParseHttpPath(c.Http.Request.URL.Path)
	request.Parser = ParseHttpInput(c.Http.Request, p.InputMaxSize)
	return &request, nil, nil
}

func (p *SSEPacker)Write(c *play.Client, output play.Output) (int, error) {
	var err error
	var data []byte
	var w = c.Http.Response

	if data, err = json.Marshal(output.All()); err != nil {
		return 0, err
	}

	if _, err = fmt.Fprintf(w, "data: %s\n\n", string(data)); err != nil {
		return 0, err
	}
	w.(http.Flusher).Flush()
	return len(data), err
}