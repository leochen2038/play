package config

import "github.com/leochen2038/play"

type config struct {
	parser interface{ play.Parser }
}

var configInstance *config

func InitConfig(parser interface{ play.Parser }) {
	configInstance = &config{parser: parser}
}

func Bool(key string) (val bool, err error) {
	var v interface{}
	if v, err = configInstance.parser.GetVal(key); err != nil {
		return
	}

	return play.ParseBool(v)
}

func String(key string) (val string, err error) {
	var v interface{}
	if v, err = configInstance.parser.GetVal(key); err != nil {
		return
	}

	return play.ParseString(v)
}

func Int(key string) (val int, err error) {
	var v interface{}
	if v, err = configInstance.parser.GetVal(key); err != nil {
		return
	}
	return play.ParseInt(v)
}

func Int64(key string) (val int64, err error) {
	var v interface{}
	if v, err = configInstance.parser.GetVal(key); err != nil {
		return
	}

	return play.ParseInt64(v)
}

func Float64(key string) (val float64, err error) {
	var v interface{}
	if v, err = configInstance.parser.GetVal(key); err != nil {
		return
	}

	return play.ParseFloat64(v)
}

func MapInterface(key string) (list map[string]interface{}, err error) {
	var v interface{}
	if v, err = configInstance.parser.GetVal(key); err != nil {
		return
	}
	return play.ParseMapInterface(v)
}
