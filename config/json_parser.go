package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

type JsonParser struct {
	data map[string]interface{}
}

func (parser *JsonParser) GetVal(key string) (val interface{}, err error) {
	keys := strings.Split(key, ".")
	lastIdx := len(keys) - 1
	searchData := parser.data

	for idx, k := range keys {
		if v, ok := searchData[k]; ok {
			val = v
			if idx != lastIdx {
				searchData = v.(map[string]interface{})
			}
		} else {
			return nil, fmt.Errorf("not exist key %q", key)
		}
	}
	return
}

func (parser *JsonParser) Update(data []byte) error {
	var newData map[string]interface{}
	err := json.Unmarshal(data, &newData)
	if err != nil {
		return err
	}
	parser.data = newData
	return nil
}
