package config

import (
	"gitlab.youban.com/go-utils/play"
)

type Parser interface {
	GetVal(key string) (val interface{}, err error)
}

type config struct {
	parser Parser
}

var configInstance *config

func InitConfig(parser interface{ Parser }) {
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
