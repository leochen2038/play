package play

type Output struct {
	data map[string]interface{}
}

func NewOutput() *Output {
	return &Output{}
}

func (o *Output) Get(key string) interface{} {
	if key != "" {
		val := o.data[key]
		return val
	}
	return o.data
}

func (o *Output) All() map[string]interface{} {
	return o.data
}

func (o *Output) Set(key string, val interface{}) {
	if o.data == nil {
		o.data = make(map[string]interface{}, 10)
	}
	o.data[key] = val
}
