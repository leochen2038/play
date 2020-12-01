package cache

import (
	"encoding/json"
	"errors"
	"github.com/leochen2038/play/middleware/etcd"
	"strings"
)

type cacheNode struct {
	ltype int
	ltime int64
	data  interface{}
}

var cacheMap map[string]map[string]interface{}
var agent *etcd.EtcdAgent
var preKey string

func InitEtcdCache(endpoints []string, appName string) {
	var err error
	preKey = "/cache/" + appName + "/"

	if agent, err = etcd.NewEtcdAgent(endpoints); err != nil {
		return
	}

	cacheMap = make(map[string]map[string]interface{})
	if data, err := agent.GetEtcdValueWithPrefix(preKey); err == nil {
		for k, v := range data {
			var tmp map[string]interface{}
			if err := json.Unmarshal(v, &tmp); err == nil {
				newkey := strings.Replace(k, preKey+"map/", "", 1)
				if newkey != k {
					cacheMap[k] = tmp
				}
			}
		}
	}

	agent.StartWatchChangeWithPrefix(preKey, func(event string, key string, data []byte) error {
		var tmp map[string]interface{}
		if err := json.Unmarshal(data, &tmp); err == nil {
			newkey := strings.Replace(key, preKey+"map/", "", 1)
			if newkey != key {
				cacheMap[newkey] = tmp
			}
		}
		return nil
	})
}

func GetMap(key string) (map[string]interface{}, error) {
	if val, ok := cacheMap[key]; !ok {
		return nil, errors.New("can not find " + key + " in cache")
	} else {
		return val, nil
	}
}

func PutMap(key string, val map[string]interface{}, appName string) error {
	var fullKey string
	if appName != "" {
		fullKey = preKey + "map/" + key
	} else {
		fullKey = "/cache/" + appName + "/map/" + key
	}

	dataByte, _ := json.Marshal(val)
	return agent.Put(fullKey, dataByte)
}
