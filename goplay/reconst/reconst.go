package reconst

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/leochen2038/play/goplay/env"
	"github.com/leochen2038/play/goplay/reconst/action"
	"github.com/leochen2038/play/goplay/reconst/meta"
)

func ReconstProject() (err error) {
	module, err := parseModuleName(os.Args[2])
	if err != nil {
		fmt.Println(err)
	}
	env.ModuleName = module
	if err = meta.MetaGenerator(); err != nil {
		return
	}
	if err = action.ReconstAction(); err != nil {
		return
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
