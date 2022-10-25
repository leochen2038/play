package gendoc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/leochen2038/play/goplay/env"
)

func GenDoc() error {
	// step 1. 初始化目录
	if err := initDir(); err != nil {
		return err
	}

	// step 2. 生成go文件
	if err := genGoCode(); err != nil {
		return err
	}

	// step 3. 通过cmd执行go文件生成文档
	if err := execGoCode(); err != nil {
		return err
	}

	return nil
}

func initDir() error {
	// step 1. 判断init文件是否存在
	if !checkExist(env.InitFileName) {
		return errors.New("re const first！")
	}

	// step 2. 判断目录是否存在
	if !checkExist(env.GenFileDir) {
		if err := os.Mkdir(env.GenFileDir, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

func checkExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}

func execGoCode() error {
	cmd := exec.Command("go", "run", env.GenFileName, env.GenFileDir+"/init.go")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	fmt.Println(string(output))

	// 删除临时文件
	_ = exec.Command("rm", "-Rf", env.GenFileName, env.GenFileDir+"/init.go").Run()
	return nil
}

func genGoCode() error {
	err := genInitFunc()
	if err != nil {
		return err
	}

	err = genMainFunc()
	if err != nil {
		return err
	}

	return nil
}

func genInitFunc() error {
	var f *os.File
	var err error

	f, err = os.OpenFile(env.GenFileDir+"/init.go", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	buf, err := os.ReadFile(env.InitFileName)
	if err != nil {
		return err
	}

	tpl := string(buf)

	_, err = f.Write([]byte(tpl))
	if err != nil {
		return err
	}

	_ = exec.Command("gofmt", "-w", env.GenFileDir+"/init.go").Run()

	return nil
}

func genMainFunc() error {
	var f *os.File
	var err error

	f, err = os.OpenFile(env.GenFileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	tpl := getGenTpl()

	_, err = f.Write([]byte(tpl))
	if err != nil {
		return err
	}

	_ = exec.Command("gofmt", "-w", env.GenFileName).Run()

	return nil
}
