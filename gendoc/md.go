package gendoc

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/leochen2038/play"
)

var mdDocument = `# 接口目录
[TOC]
# 接口详情
`

var mdTemplate = `
## {{name}}

### 接口描述 {{desc}}
>  请求参数

| 参数名称 | 类型 | 必填 | 描述 | 默认 |
|------|------|------|-----|-----|
{{request}}
>  响应参数

| **字段** | **类型** | **必须** | **备注**  |
|------|------|------|-----|
{{response}}
`

func genMkd() error {
	// step 1. 打开文件
	var f *os.File
	var err error
	f, err = os.OpenFile(mkdFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	// step 2. 获取内容
	err = play.WalkAction(getMdActionTpl)
	if err != nil {
		fmt.Println(err)
		return err
	}

	// step 3. 写入文件
	_, err = f.Write([]byte(mdDocument))
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func getMdActionTpl(action *play.Action) error {
	var tmp = mdTemplate

	tmp = strings.ReplaceAll(tmp, "{{name}}", action.Name())
	tmp = strings.ReplaceAll(tmp, "{{desc}}", action.MetaData()["desc"])
	tmp = strings.ReplaceAll(tmp, "{{request}}", getMdFieldTpl(action.Input(), 0))
	tmp = strings.ReplaceAll(tmp, "{{response}}", getMdFieldTpl(action.Output(), 0))
	mdDocument += tmp

	return nil
}

func getMdFieldTpl(fields map[string]play.ActionField, level int) string {
	var tmp string
	var names []string
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		field := fields[name]
		required := "否"
		if field.Required {
			required = "是"
		}
		fieldName := field.Field
		if len(field.Keys) > 0 {
			fieldName = strings.Join(field.Keys, ",")
		}
		if level > 0 {
			fieldName = strings.Repeat("&nbsp;&nbsp;", level) + "└ " + fieldName
		}
		tmp += fmt.Sprintf("| %s | %s | %s | %s | %s | \n", fieldName, field.Typ, required, field.Desc, field.Default)
		if field.Child != nil {
			tmp += getMdFieldTpl(field.Child, level+1)
		}
	}
	return tmp
}
