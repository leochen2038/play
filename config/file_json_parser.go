package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type FileJsonParser struct {
	refreshTickTime time.Duration
	lastFileModTime int64
	filename        string
	data            map[string]interface{}
}

func (parser *FileJsonParser) GetVal(key string) (val interface{}, err error) {
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
			return nil, errors.New("not exist key " + key)
		}
	}
	return
}

func NewFileJsonParser(file string, refresh time.Duration) (Parser, error) {
	var err error
	var dataByte []byte
	var parser = new(FileJsonParser)

	if dataByte, err = os.ReadFile(file); err != nil {
		return nil, err
	}

	if err = json.Unmarshal(dataByte, &parser.data); err != nil {
		return nil, err
	}

	fileInfo, _ := os.Stat(file)
	parser.filename = file
	parser.refreshTickTime = refresh
	parser.lastFileModTime = fileInfo.ModTime().Unix()

	if refresh > 0 {
		parser.refreshTickTime = refresh
		parser.startWatchFile()
	}

	return parser, nil
}

func (parser *FileJsonParser) startWatchFile() {
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

func (parser *FileJsonParser) watchFile() {
	var err error
	var fileInfo os.FileInfo
	var ticker = time.NewTicker(parser.refreshTickTime)

	for range ticker.C {
		if fileInfo, err = os.Stat(parser.filename); err == nil && fileInfo.ModTime().Unix() > parser.lastFileModTime {
			if dataByte, err := os.ReadFile(parser.filename); err != nil {
				var tmp map[string]interface{}
				parser.lastFileModTime = fileInfo.ModTime().Unix()
				if err := json.Unmarshal(dataByte, &tmp); err != nil {
					parser.data = tmp
				}
			}
		}
	}
}
