package config

import (
	"fmt"
	"os"
	"time"
)

type FileJsonParser struct {
	refreshTickTime time.Duration
	lastFileModTime int64
	filename        string
	data            JsonParser
}

func (parser *FileJsonParser) GetVal(key string) (val interface{}, err error) {
	return parser.data.GetVal(key)
}

func NewFileJsonParser(file string, refresh time.Duration) (Parser, error) {
	var err error
	var dataByte []byte
	var parser = new(FileJsonParser)

	if dataByte, err = os.ReadFile(file); err != nil {
		return nil, err
	}

	if err = parser.data.Update(dataByte); err != nil {
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
			if dataByte, err := os.ReadFile(parser.filename); err == nil {
				parser.data.Update(dataByte)
				parser.lastFileModTime = fileInfo.ModTime().Unix()
			}
		}
	}
}
