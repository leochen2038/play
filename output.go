package play

import (
	"gitlab.youban.com/go-utils/play/codec/renders"
)

type Output struct {
	render renders.Render
	data   map[string]interface{}
}

func (o *Output) Get(key string) interface{} {
	if key != "" {
		val, _ := o.data[key]
		return val
	}
	return o.data
}

func (o *Output) All() interface{} {
	return o.data
}

func (o *Output) Set(key string, val interface{}) {
	if o.data == nil {
		o.data = make(map[string]interface{}, 10)
	}
	o.data[key] = val
}

func (o Output) Render() ([]byte, error) {
	return o.render.Render(o.data)
}

func (o Output) RenderName() string {
	return o.render.Name()
}

func (o *Output) SetRender(render renders.Render) {
	o.render = render
}
