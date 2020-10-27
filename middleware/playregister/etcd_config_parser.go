package playregister

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/leochen2038/play"
	"go.etcd.io/etcd/clientv3"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

type EtcdParser struct {
	configKey  string
	configFile string
	etcdClient *clientv3.Client
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

func NewEtcdParser(endpoints []string, configKey string) (parser play.Parser, err error) {
	var etcdClient *clientv3.Client
	var data map[string]interface{}
	var etcdParser EtcdParser
	var dataByte []byte

	defer func() {
		if err != nil {
			parser, err = etcdParser.tryGetConfigFromLocalFile(err)
		}
		if err == nil && etcdParser.etcdClient != nil {
			go etcdParser.watchEtcdConfig()
		}
	}()

	etcdParser.configKey = configKey
	if path, err := os.Executable(); err == nil {
		etcdParser.configFile = path + ".json"
	}

	if len(endpoints) == 0 {
		return nil, errors.New("empty endpoints")
	}
	if configKey == "" {
		return nil, errors.New("empty etcd configkey")
	}

	if etcdClient, err = clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		DialTimeout:          100 * time.Millisecond,
		DialKeepAliveTimeout: 1 * time.Second},
	); err != nil {
		return
	}

	etcdParser.etcdClient = etcdClient
	if dataByte, err = etcdParser.getEtcdValue(configKey); err != nil {
		return
	}
	if err = json.Unmarshal(dataByte, &data); err != nil {
		return
	}

	etcdParser.data = data
	return &etcdParser, nil
}

func (e *EtcdParser) getEtcdValue(key string) (data []byte, err error) {
	ctx, cancelFunc := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	resp, err := e.etcdClient.Get(ctx, key)
	if cancelFunc(); err != nil {
		return
	}

	for _, kv := range resp.Kvs {
		if string(kv.Key) == key {
			return kv.Value, nil
		}
	}

	return nil, errors.New("unable get " + key)
}

func (e *EtcdParser) watchEtcdConfig() {
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			log.Println("[playregister]", panicInfo)
		}
		time.Sleep(5 * time.Second)
		go e.watchEtcdConfig()
	}()

	ctx, _ := context.WithCancel(context.TODO())
	watchChan := e.etcdClient.Watch(ctx, e.configKey)

	for {
		select {
		case watchResp := <-watchChan:
			for _, event := range watchResp.Events {
				if event.Type == clientv3.EventTypePut {
					if err := e.updateConfig(event.Kv.Value); err != nil {
						log.Println("[playregister]", err)
					}
				}
			}
		}
	}
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

func (e *EtcdParser) updateConfig(data []byte) (err error) {
	var obj map[string]interface{}
	if err = json.Unmarshal(data, &obj); err == nil {
		log.Println("[playregister service config change]", string(data))
		e.data = obj
	}

	if e.configFile != "" {
		_ = ioutil.WriteFile(e.configFile, data, 0644)
	}

	return
}
