package play

type Parser interface {
	GetVal(key string) (val interface{}, err error)
	Bind(obj interface{}) error
}

type Input struct {
	parser interface{ Parser }
}

func NewInput(parser interface{ Parser }) *Input {
	return &Input{parser: parser}
}

func (in *Input) Bind(obj interface{}) (err error) {
	return in.parser.Bind(obj)
}

func (in *Input) Interface(key string) (interface{}, error) {
	return in.parser.GetVal(key)
}

func (in Input) SliceString(key string) (list []string, err error) {
	var v interface{}
	if v, err = in.parser.GetVal(key); err != nil {
		return
	}
	return ParseSliceString(v)
}

func (in Input) SliceInterface(key string) (list []interface{}, err error) {
	var v interface{}
	if v, err = in.parser.GetVal(key); err != nil {
		return
	}
	return ParseSliceInterface(v)
}

func (in Input) MapInterface(key string) (list map[string]interface{}, err error) {
	var v interface{}
	if v, err = in.parser.GetVal(key); err != nil {
		return
	}
	return ParseMapInterface(v)
}

func (in Input) Bool(key string) (val bool, err error) {
	var v interface{}
	if v, err = in.parser.GetVal(key); err != nil {
		return
	}
	return ParseBool(v)
}

func (in Input) String(key string) (val string, err error) {
	var v interface{}
	if v, err = in.parser.GetVal(key); err != nil {
		return
	}
	return ParseString(v)
}

func (in Input) Int(key string) (val int, err error) {
	var v interface{}
	if v, err = in.parser.GetVal(key); err != nil {
		return
	}
	return ParseInt(v)
}

func (in Input) Int8(key string) (val int8, err error) {
	var v interface{}
	if v, err = in.parser.GetVal(key); err != nil {
		return
	}
	return ParseInt8(v)
}

func (in Input) Int64(key string) (val int64, err error) {
	var v interface{}
	if v, err = in.parser.GetVal(key); err != nil {
		return
	}
	return ParseInt64(v)
}

func (in Input) Float32(key string) (val float32, err error) {
	var v interface{}
	if v, err = in.parser.GetVal(key); err != nil {
		return
	}
	return ParseFloat32(v)
}

func (in Input) Float64(key string) (val float64, err error) {
	var v interface{}
	if v, err = in.parser.GetVal(key); err != nil {
		return
	}
	return ParseFloat64(v)
}
