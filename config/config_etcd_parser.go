package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/middleware/etcd"
	"io/ioutil"
	"os"
	"strings"
)

type EtcdParser struct {
	configKey  string
	configFile string
	etcdAgent  *etcd.EtcdAgent
	data       map[string]interface{}
}

func (e *EtcdParser) GetVal(key string) (val interface{}, err error) {
	keys := strings.Split(key, ".")
	lastIdx := len(keys) - 1
	searchData := e.data

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

func (e *EtcdParser) Bind(obj interface{}) error {
	panic("EtcdParser no support Bind Method yet")
}

func NewEtcdParser(etcd *etcd.EtcdAgent, configKey string) (parser play.Parser, err error) {
	var etcdParser EtcdParser
	var dataByte []byte

	defer func() {
		if err != nil {
			parser, err = etcdParser.tryGetConfigFromLocalFile(err)
		}
	}()

	etcdParser.etcdAgent = etcd
	etcdParser.configKey = configKey
	if path, err := os.Executable(); err == nil {
		etcdParser.configFile = path + ".json"
	}

	if etcd == nil || len(etcd.Endpoints) == 0 {
		return nil, errors.New("empty endpoints")
	}
	if configKey == "" {
		return nil, errors.New("empty etcd configkey")
	}

	defer etcdParser.etcdAgent.StartWatchChange(configKey, func(data []byte) (err error) {
		return etcdParser.update(data)
	})

	if dataByte, err = etcd.GetEtcdValue(configKey); err != nil {
		return
	}

	if err = etcdParser.update(dataByte); err != nil {
		return
	}

	return &etcdParser, nil
}

func (e *EtcdParser) tryGetConfigFromLocalFile(orginErr error) (*EtcdParser, error) {
	var err error
	var data []byte
	var obj map[string]interface{}

	if e.configFile == "" {
		return nil, errors.New(orginErr.Error() + " and not find local config file")
	}

	if data, err = ioutil.ReadFile(e.configFile); err == nil {
		if err = json.Unmarshal(data, &obj); err == nil {
			e.data = obj
		}
	}

	if err != nil {
		return nil, errors.New(orginErr.Error() + " and " + err.Error())
	}

	return e, nil
}

func (e *EtcdParser) update(data []byte) (err error) {
	var obj map[string]interface{}
	if err = json.Unmarshal(data, &obj); err != nil {
		return
	}

	e.data = obj
	if e.configFile != "" {
		_ = ioutil.WriteFile(e.configFile, data, 0644)
	}

	return
}
