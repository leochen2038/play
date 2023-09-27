package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/leochen2038/play/goplay/env"
	"github.com/leochen2038/play/goplay/gendoc"
	"github.com/leochen2038/play/goplay/initProject"
	"github.com/leochen2038/play/goplay/reconst"
)

var command string

// var commandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

// 多包同名，可以在import进行引用别名
func init() {
	env.FrameworkVer = "v0.7.6"
	env.FrameworkName = "github.com/leochen2038/play"
	// commandLine.StringVar(&env.FrameworkName, "f", "github.com/leochen2038/play", "framework module")
	// commandLine.Parse(os.Args[2:])
	// commandLine.Parse(os.Args[2:])

	if len(os.Args) < 2 {
		fmt.Printf(`goplay version: %s
Usage:
	play <command> <arguments>

The commands are:
	init	init a new project
	reconst	project path
    gendoc  generate api document`, env.FrameworkVer)
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
	env.ActionDefaultTimeout = "500"
	env.InitFileName = env.ProjectPath + "/init.go"
	env.GenFileDir = env.ProjectPath + "/doc.md"
	env.GenFileName = env.GenFileDir + "/main.go"
}

func main() {
	fmt.Printf(`
     ____     __      ___  __  __
    / __ \   / /     /   | \ \/ /
   / /_/ /  / /     / /| |  \  / 
  / ____/  / /___  / ___ |  / /  
 /_/      /_____/ /_/  |_| /_/ %s 

`, env.FrameworkVer)
	switch command {
	case "init":
		if err := initProject.InitProject(false); err != nil {
			fmt.Println("init project error ", err)
		}
	case "reconst":
		if err := reconst.ReconstProject(); err != nil {
			fmt.Println(err)
		}
	case "gendoc":
		if err := gendoc.GenDoc(); err != nil {
			fmt.Println(err)
		}
	default:
		fmt.Println("unknow command:", command)
	}
}
