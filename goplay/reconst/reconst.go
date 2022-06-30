package reconst

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"gitlab.youban.com/go-utils/play/goplay/reconst/action"
	"gitlab.youban.com/go-utils/play/goplay/reconst/meta"
)

func ReconstProject() (err error) {
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
