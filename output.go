package play

type Output interface {
	Get(key string) interface{}
	Set(key string, val interface{})
	All() interface{}
}

type KvOutput struct {
	data map[string]interface{}
}

func (o *KvOutput) Get(key string) interface{} {
	if key != "" {
		val, _ := o.data[key]
		return val
	}
	return o.data
}

func (o *KvOutput) All() interface{} {
	return o.data
}

func (o *KvOutput) Set(key string, val interface{}) {
	if o.data == nil {
		o.data = make(map[string]interface{}, 10)
	}
	o.data[key] = val
}
