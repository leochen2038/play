package renders

import "github.com/leochen2038/play/codec/protos/golang/json"

var jRender = &jsonRender{}

type jsonRender struct {
}

func GetRenderOfJson() Render {
	return jRender
}

func (r jsonRender) Name() string {
	return "json"
}

func (r jsonRender) Render(data map[string]interface{}) ([]byte, error) {
	return json.MarshalEscape(data, false, false)
}
