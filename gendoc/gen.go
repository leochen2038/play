package gendoc

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var sdkFileDir string
var mkdFileDir string
var sdkFilePath string
var mkdFilePath string

var gendoc = flag.NewFlagSet("gendoc", flag.ContinueOnError)

func init() {
	execPath, _ := os.Executable()
	pwdPath, _ := os.Getwd()
	projectName := filepath.Base(filepath.Dir(execPath))

	gendoc.StringVar(&sdkFileDir, "sdk", "", "sdk文件目录")
	gendoc.StringVar(&mkdFileDir, "mkd", "", "mkd文件目录")
	_ = gendoc.Parse(os.Args[1:])

	if sdkFileDir == "." || sdkFileDir == "./" {
		sdkFilePath = pwdPath + "/" + projectName + ".sdk"
	} else if sdkFileDir != "" {
		sdkFilePath = sdkFileDir + "/" + projectName + "/" + projectName + ".go"
	}
	if mkdFileDir == "." || mkdFileDir == "./" {
		mkdFilePath = pwdPath + "/" + projectName + ".md"
	} else if mkdFileDir != "" {
		mkdFilePath = mkdFileDir + "/" + projectName + ".md"
	}
}

func GenSdkAndMkd() {
	var err error
	var genFlag bool

	if sdkFilePath != "" {
		genFlag = true
		fmt.Println("start gen sdk...")
		err = genSdk()
		if err != nil {
			fmt.Println("gen sdk failed:", err)
		} else {
			fmt.Println("gen sdk success")
		}
	}
	if mkdFilePath != "" {
		genFlag = true
		fmt.Println("start gen mkd...")
		err = genMkd()
		if err != nil {
			fmt.Println("gen mkd failed:", err)
		} else {
			fmt.Println("gen mkd success")
		}
	}

	if genFlag {
		os.Exit(0)
	}
}
