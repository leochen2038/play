package play

type Output interface {
	Get(key string) interface{}
	Set(key string, val interface{})
}

type playKvOutput struct {
	data map[string]interface{}
}

func (o *playKvOutput) Set(key string, val interface{}) {
	if o.data == nil {
		o.data = make(map[string]interface{}, 10)
	}
	o.data[key] = val
}

func (o *playKvOutput) Get(key string) interface{} {
	if key != "" {
		val, _ := o.data[key]
		return val
	}
	return o.data
}
