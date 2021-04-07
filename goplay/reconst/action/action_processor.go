package action

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/leochen2038/play/goplay/reconst/env"
	"io/ioutil"
	"os"
	"strings"
)

func checkProcessorFile(name string) (err error) {
	v := strings.ReplaceAll(name, ".", "/") // 有bug可能没有目录
	idx := strings.LastIndex(v, "/")
	file := env.ProjectPath + "/processor/" + v + ".go"
	if idx < 0 {
		return errors.New("error syntax at " + name)
	}
	path := env.ProjectPath + "/processor/" + v[:idx]

	pacekageNme := path[strings.LastIndex(path, "/")+1:]
	frameworkPath := "github.com/leochen2038/play"
	funcName := v[idx+1:]
	_, err = os.Stat(file)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(path, 0744); err != nil {
			return
		}
		if _, err = os.Create(file); err != nil {
			return
		}

		src := getProcessorTpl(pacekageNme, frameworkPath, funcName)
		if err = ioutil.WriteFile(file, []byte(src), 0644); err != nil {
			return
		}
		fmt.Printf("create processor %s\n", file)
	}
	return
}

func parseModuleName(path string) (string, error) {
	modPath := fmt.Sprintf("%s/go.mod", path)
	_, err := os.Stat(modPath)
	if os.IsNotExist(err) {
		return "", errors.New("can not find go.mod in project")
	}

	file, err := os.Open(modPath)
	br := bufio.NewReader(file)
	data, _, err := br.ReadLine()

	return strings.Split(string(data), " ")[1], nil
}
