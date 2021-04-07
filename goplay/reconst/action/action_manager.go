package action

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type action struct {
	name        string
	handlerList *processorHandler
}

type processorHandler struct {
	name     string
	rcstring string
	parent   *processorHandler
	next     []*processorHandler
}

var actions = make(map[string]action, 32)

func getActions(path string) (map[string]action, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}

	if err := initActions(path); err != nil {
		return nil, err
	}
	return actions, nil
}

func initActions(path string) error {
	err := filepath.Walk(path, func(filename string, fi os.FileInfo, err error) error {
		if !fi.IsDir() && fi.Name()[0:1] != "." {
			d, err := ioutil.ReadFile(filename)
			if err != nil {
				return err
			}

			tokens, err := parseTokenFrom(bytes.NewReader(d), filename)
			if err != nil {
				return err
			}
			if err = buildActions(tokens); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// 根据token列表，构建出action结构
func buildActions(tokens []string) error {
	var curp *processorHandler = nil
	for i := 0; i < len(tokens); i++ {
		v := tokens[i]
		if v == "{" && curp == nil && i != 0 {
			i += 1
			v = tokens[i]
			if v == "}" {
				curp = nil
			} else {
				curp = &processorHandler{}
				curp.name = v
			}

			for _, iv := range strings.Split(tokens[i-2], ",") {
				action := action{name: iv, handlerList: curp}
				actions[iv] = action
			}
			continue
		}

		if v == "(" && curp != nil {
			i += 1
			rc := tokens[i]
			if rc == ")" {
				curp = curp.parent
				continue
			}

			i += 1
			v = tokens[i]
			proc := &processorHandler{}
			proc.parent = curp
			proc.rcstring = rc
			proc.name = v
			curp.next = append(curp.next, proc)
			curp = proc
			continue
		}

		if v == ")" && curp != nil {
			curp = curp.parent
			continue
		}

		if curp != nil {
			rc := v
			i += 1
			v = tokens[i]
			proc := &processorHandler{}
			proc.parent = curp
			proc.rcstring = rc
			proc.name = v
			curp.next = append(curp.next, proc)
			curp = proc
			continue
		}

		if v == "}" {
			curp = nil
			continue
		}
	}
	return nil
}

// 从输入流里分析出token
func parseTokenFrom(reader *bytes.Reader, filename string) ([]string, error) {
	token := make([]byte, 0, 32)
	tokens := make([]string, 0, 128)

	for {
		c, err := reader.ReadByte()
		if err != nil {
			break
		}
		if c == ',' {
			if len(token) > 0 {
				token = append(token, c)
			}
			continue
		}
		if c == '#' {
			for ; c != '\n'; c, err = reader.ReadByte() {
				if err != nil {
					break
				}
			}
			continue
		}
		if c == '>' {
			if len(token) > 0 {
				tokens = append(tokens, string(token))
				token = token[0:0]
				continue
			}
			return nil, errors.New("miss return define befer => ")
		}
		if c == '{' || c == '(' {
			if len(token) == 0 {
				return nil, errors.New("miss action name or processor define befer '{' or '(' by parse:" + filename)
			}
			tokens = append(tokens, string(token))
			tokens = append(tokens, string(c))
			token = token[0:0]
			continue
		}
		if c == '}' || c == ')' {
			if len(token) > 0 {
				tokens = append(tokens, string(token))
				token = token[0:0]
			}
			tokens = append(tokens, string(c))
			continue
		}
		if c != '\n' && c != '\t' && c != ' ' && c != '\r' && c != '-' && c != '=' {
			token = append(token, c)
		}
	}

	return tokens, nil
}
