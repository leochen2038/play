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

func InitCacheWithEtcdAgent(etcdAgent *etcd.EtcdAgent, appName string) {
	preKey = "/cache/" + appName + "/"
	agent = etcdAgent

	cacheMap = make(map[string]map[string]interface{})
	if data, err := agent.GetEtcdValueWithPrefix(preKey); err == nil {
		for k, v := range data {
			var tmp map[string]interface{}
			if err := json.Unmarshal(v, &tmp); err == nil {
				newkey := strings.Replace(k, preKey+"map/", "", 1)
				if newkey != k {
					cacheMap[newkey] = tmp
				}
			}
		}
	}

	agent.StartWatchChangeWithPrefix(preKey, func(event string, key string, data []byte) error {
		if event == "put" {
			var tmp map[string]interface{}
			if err := json.Unmarshal(data, &tmp); err == nil {
				newkey := strings.Replace(key, preKey+"map/", "", 1)
				if newkey != key {
					cacheMap[newkey] = tmp
				}
			}
		} else if event == "del" {
			newkey := strings.Replace(key, preKey+"map/", "", 1)
			if newkey != key {
				delete(cacheMap, newkey)
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

func PutMap(key string, val map[string]interface{}) error {
	fullKey := preKey + "map/" + key
	dataByte, _ := json.Marshal(val)
	return agent.Put(fullKey, dataByte)
}

func DelMap(key string) error {
	fullKey := preKey + "map/" + key
	return agent.Del(fullKey)
}
