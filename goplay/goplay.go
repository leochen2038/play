package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/leochen2038/play/goplay/initProject"
	"github.com/leochen2038/play/goplay/reconst"
	"github.com/leochen2038/play/goplay/reconst/env"
	"os"
	"runtime"
	"strings"
)

var command string

// 多包同名，可以在import进行引用别名
func init() {
	env.FrameworkName = "github.com/leochen2038/play"
	env.FrameworkVer = "v0.6.8"

	if len(os.Args) < 2 {
		fmt.Printf(`goplay version: %s
Usage:
	play <command> <arguments>

The commands are:
	init	init a new project
	reconst	project path`, env.FrameworkVer)
		os.Exit(1)
	}
	if len(os.Args) < 3 {
		fmt.Println("please input project path")
		os.Exit(1)
	}

	command = os.Args[1]
	env.ProjectPath = os.Args[2]
	env.GoVersion = runtime.Version()[2:]
	env.WithoutModuleName = 0
}

func main() {
	switch command {
	case "init":
		if err := initProject.InitProject(false); err != nil {
			fmt.Println("init project error ", err)
		}
	case "reconst":
		module, err := parseModuleName(os.Args[2])
		if err != nil {
			fmt.Println(err)
		}

		env.ModuleName = module
		if err := reconst.ReconstProject(); err != nil {
			fmt.Println(err)
		}
	default:
		fmt.Println("unknow command:", command)
	}
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
