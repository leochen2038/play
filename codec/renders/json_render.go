package renders

import "gitlab.youban.com/go-utils/play/library/golang/json"

var jRender = &jsonRender{}

type jsonRender struct {
}

func GetJsonRender() Render {
	return jRender
}

func (r jsonRender) Name() string {
	return "json"
}

func (r jsonRender) Render(data map[string]interface{}) ([]byte, error) {
	return json.MarshalEscape(data, false, false)
}