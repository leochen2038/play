package config

import (
	"encoding/json"
	"fmt"
	"github.com/leochen2038/play"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

type JsonParser struct {
	refashTickTime  time.Duration
	lastFileModTime int64
	filename        string
	data            map[string]interface{}
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

func (parser *JsonParser) Bind(obj interface{}) error {
	panic("EtcdParser no support Bind Method yet")
}

func NewJsonParser(file string, refashTickTime time.Duration) (parser play.Parser, err error) {
	var dataByte []byte
	var jsonParser = JsonParser{}

	if dataByte, err = ioutil.ReadFile(file); err != nil {
		return nil, err
	}

	if err = json.Unmarshal(dataByte, &jsonParser.data); err != nil {
		return nil, err
	}

	fileInfo, _ := os.Stat(file)
	jsonParser.filename = file
	jsonParser.refashTickTime = refashTickTime
	jsonParser.lastFileModTime = fileInfo.ModTime().Unix()

	if refashTickTime > 0 {
		jsonParser.refashTickTime = refashTickTime
		jsonParser.startWatchFile()
	}

	return &jsonParser, nil
}

func (parser *JsonParser) startWatchFile() {
	go func() {
		defer func() {
			if panicInfo := recover(); panicInfo != nil {
				fmt.Println("start watch config file panic:", panicInfo)
			}
			time.Sleep(5 * time.Second)
			parser.startWatchFile()
		}()
		parser.watchFile()
	}()
}

func (parser *JsonParser) watchFile() {
	var err error
	var fileInfo os.FileInfo
	var ticker = time.NewTicker(parser.refashTickTime * time.Second)

	for {
		select {
		case <-ticker.C:
			if fileInfo, err = os.Stat(parser.filename); err == nil && fileInfo.ModTime().Unix() > parser.lastFileModTime {
				var tmp map[string]interface{}
				dataByte, _ := ioutil.ReadFile(parser.filename)
				parser.lastFileModTime = fileInfo.ModTime().Unix()
				if err := json.Unmarshal(dataByte, &tmp); err != nil {
					parser.data = tmp
				}
			}
		}
	}
}
