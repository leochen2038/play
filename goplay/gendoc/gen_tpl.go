package gendoc

import (
	"fmt"

	"github.com/leochen2038/play/goplay/env"
)

func getGenTpl() string {
	return fmt.Sprintf(`
package main

import (
	"fmt"
	"github.com/leochen2038/play"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"sync"
)

var ActionDescMap map[string]string

func main() {

	var mdTpl string
	mdFileMame := "%s"

	// step 1. 创建md文件
	f, err := os.OpenFile(mdFileMame, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		fmt.Println(err)
	}
	defer func() {
		_ = f.Close()
	}()

	mdTpl = fmt.Sprintf("# 接口提纲\n")
	mdTpl += fmt.Sprintf("[TOC] \n")
	mdTpl += fmt.Sprintf("# 接口详情\n")

	// step 2. 编写文档
	for actionUri, action := range play.GetActionPools() {
		mdTpl += fmt.Sprintf(getActionTpl(actionUri, action) + "\n")
	}

	// step 3. 写入文件
	_, err = f.Write([]byte(mdTpl))
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("doc generate ok!")
}

func getActionTpl(actionUri string, action *sync.Pool) string {

	var mdTpl string
	actionDoc, err := play.ParseAction(action.Get().(*play.ProcessorWrap))
	if err != nil {
		fmt.Println(err)
		return mdTpl
	}

	err = getActionDesc()
	if err != nil {
		fmt.Println(err)
		return mdTpl
	}

	mdTpl += fmt.Sprintf("## /" + strings.Replace(actionUri, ".", "/", -1) + "\n")

	if desc, ok := ActionDescMap[actionUri]; ok {
		mdTpl += fmt.Sprintf("### 接口描述\n")
		mdTpl += fmt.Sprintf("%%s\n", desc)
	}

	mdTpl += fmt.Sprintf("### 请求参数\n")
	mdTpl += fmt.Sprintf("| 参数名称 | 类型 | 必填 | 描述 | 默认 | \n")
	mdTpl += fmt.Sprintf("|------|------|------|-----|-----| \n")

	for _, r := range actionDoc.Request {
		required := "否"
		if r.Required {
			required = "是"
		}
		mdTpl += fmt.Sprintf("| %%s | %%s | %%s | %%s | %%s | \n", getFullName(r, r.Father), r.Typ, required, r.Description, r.Default)
	}

	mdTpl += fmt.Sprintf("### 响应参数\n")
	mdTpl += fmt.Sprintf("| **字段** | **类型** | **必须** | **备注**  | \n")
	mdTpl += fmt.Sprintf("|------|------|------|-----| \n")

	for _, r := range actionDoc.Response {
		required := "否"
		if r.Required {
			required = "是"
		}
		mdTpl += fmt.Sprintf("| %%s | %%s | %%s | %%s | \n", getFullName(r, r.Father), r.Typ, required, r.Description)
	}

	return mdTpl
}

func getFullName(field *play.ActionField, father *play.ActionField) string {
	name := field.Field

	if father == nil {
		return name
	}

	return getFullName(field.Father, field.Father.Father) + "." + name
}

func getActionDesc() error {
	files := foreachDir("%s")

	for _, file := range files {
		buf, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		reg, _ := regexp.Compile("(#(.*?)\r\n(.*?){)+")
		matches := reg.FindAllStringSubmatch(string(buf), -1)

		for _, match := range matches {
			if len(match) == 4 {
				if ActionDescMap == nil {
					ActionDescMap = make(map[string]string)
				}
				ActionDescMap[strings.Trim(match[3], " ")] = strings.Trim(match[2], " ")
			}
		}
	}

	return nil
}

func foreachDir(dirPth string) []string {
	files := make([]string, 0)

	dir, err := ioutil.ReadDir(dirPth)
	if err != nil {
		fmt.Println(err)
		return files
	}

	PthSep := string(os.PathSeparator)

	for _, fi := range dir {
		if fi.IsDir() {
			files = append(files, foreachDir(dirPth+PthSep+fi.Name())...)
		}
		if fi.Name() != "." && fi.Name() != ".." && !fi.IsDir() { //匹配文件
			files = append(files, dirPth+PthSep+fi.Name())
		}
	}

	return files
}

`, env.GenFileDir+"/doc.md", env.ProjectPath+"/assets/action")
}
