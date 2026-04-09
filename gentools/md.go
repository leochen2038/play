package gentools

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/leochen2038/play"
)

var tocTemplate = `
[{{title}}](#{{link}})
`
var apiTemplate = `
# {{name}}

> 接口描述 {{desc}}
## 请求参数

| 参数名称 | 类型 | 必填 | 描述 | 默认 |
|------|------|------|-----|-----|
{{request}}
## 响应参数

| 参数名称 | 类型 | 描述  |
|------|------|-----|
{{response}}

> 响应示例

{{example}}
`

func GenMdDocs(path string, is ...play.IServer) (err error) {
	for _, i := range is {
		// step 1. 打开文件
		var f *os.File
		var mktoc, mkapi string
		filePath := fmt.Sprintf("%s/%s.md", path, i.Info().Name())
		if f, err = os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755); err != nil {
			return
		}
		defer func() {
			_ = f.Close()
		}()

		// step 2. 获取内容
		names := i.ActionUnitNames()
		for _, name := range names {
			toc, api := getMdActionTpl(i.LookupActionUnit(name))
			mktoc += toc
			mkapi += api
		}

		// step 3. 写入文件
		if _, err = f.Write([]byte(mktoc + "\n\n" + mkapi)); err != nil {
			return
		}
	}
	return
}

func getMdActionTpl(action *play.ActionUnit) (toc, api string) {
	var tmp = apiTemplate
	var tocTmp = tocTemplate
	tocTmp = strings.ReplaceAll(tocTmp, "{{title}}", action.RequestName)
	tocTmp = strings.ReplaceAll(tocTmp, "{{link}}", action.RequestName)
	tmp = strings.ReplaceAll(tmp, "{{name}}", action.RequestName)
	tmp = strings.ReplaceAll(tmp, "{{desc}}", action.Action.MetaData()["desc"])
	tmp = strings.ReplaceAll(tmp, "{{request}}", getMdFieldTplInput(action.Action.Input(), 0))
	tmp = strings.ReplaceAll(tmp, "{{response}}", getMdFieldTplOutput(action.Action.Output(), 0))
	tmp = strings.ReplaceAll(tmp, "{{example}}", "```json\n"+action.Action.Example()+"\n```")

	return tocTmp, tmp
}

func getMdFieldTplInput(fields map[string]play.ActionField, level int) string {
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
			tmp += getMdFieldTplInput(field.Child, level+1)
		}
	}
	return tmp
}

func getMdFieldTplOutput(fields map[string]play.ActionField, level int) string {
	var tmp string
	var names []string
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		field := fields[name]
		fieldName := field.Field
		if len(field.Keys) > 0 {
			fieldName = strings.Join(field.Keys, ",")
		}
		if level > 0 {
			fieldName = strings.Repeat("&nbsp;&nbsp;", level) + "└ " + fieldName
		}
		tmp += fmt.Sprintf("| %s | %s | %s | \n", fieldName, field.Typ, field.Desc)
		if field.Child != nil {
			tmp += getMdFieldTplOutput(field.Child, level+1)
		}
	}
	return tmp
}
