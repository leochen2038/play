package gentools

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/leochen2038/play"
)

var sdkDocument = fmt.Sprintf(`
package {{packageName}}

import (
	"context"
	"errors"
	"github.com/leochen2038/play"
)

type CommonRsp struct {
	Rc  int64   %sjson:"rc"%s
	Msg string  %sjson:"msg"%s
	Tm  int64   %sjson:"tm"%s
}

type PrimitiveObjectID int64

`, "`", "`", "`", "`", "`", "`")

var sdkTemplate = `
type {{requestName}} struct {
	{{requestFields}}
}

type {{responseName}} struct {
	CommonRsp
	{{responseFields}}
}

{{specialFields}}

// {{callAction}} {{desc}}
func {{callAction}}(ctx context.Context, agent play.Agent, req {{requestName}}) (resp {{responseName}}, err error) {
	var service, action = "{{moduleName}}", "{{actionName}}"
	var sendData, recvData []byte

	if sendData, err = agent.Marshal(ctx, service, action, req); err != nil {
		return
	}
	if recvData, err = agent.Request(ctx, service, action, sendData); err != nil {
		return
	}
	if err = agent.Unmarshal(ctx, service, action, recvData, &resp); err != nil {
		return
	}
	if resp.Rc != 0 {
		return resp, errors.New(resp.Msg)
	}
	return
}
`

var sdkField = `
type {{name}} struct {
	{{fields}}
}
`

var specialFields = map[string]string{}

func GenSdk(path string, is ...play.IServer) (err error) {
	for _, i := range is {
		filePath := fmt.Sprintf("%s/%s_sdk.go", path, i.Info().Name())

		// step 1. 打开文件
		var f *os.File
		if f, err = os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755); err != nil {
			return
		}
		defer func() {
			_ = f.Close()
		}()

		// step 2. 获取内容
		actions := i.ActionUnitNames()
		for _, action := range actions {
			if err = getSdkActionTpl(i.Info().Name(), i.LookupActionUnit(action)); err != nil {
				return
			}
		}

		// step 3. 写入文件
		if _, err = f.Write([]byte(sdkDocument)); err != nil {
			return
		}
		_ = exec.Command("gofmt", "-w", filePath).Run()
	}
	return
}

func getSdkActionTpl(moduleName string, unit *play.ActionUnit) (err error) {
	var tmp = sdkTemplate
	sdkDocument = strings.ReplaceAll(sdkDocument, "{{packageName}}", getPackageName(moduleName))

	actionSpecialFields := map[string]string{}
	requestAction := getRequestAction(unit.RequestName)
	callAction := "Call" + requestAction

	tmp = strings.ReplaceAll(tmp, "{{requestName}}", "Req"+requestAction)
	tmp = strings.ReplaceAll(tmp, "{{responseName}}", "Res"+requestAction)
	tmp = strings.ReplaceAll(tmp, "{{moduleName}}", moduleName)
	tmp = strings.ReplaceAll(tmp, "{{desc}}", unit.Action.MetaData()["desc"])
	tmp = strings.ReplaceAll(tmp, "{{actionName}}", unit.RequestName)
	tmp = strings.ReplaceAll(tmp, "{{callAction}}", callAction)
	tmp = strings.ReplaceAll(tmp, "{{requestFields}}", getSdkFieldTpl(unit.Action.Input(), requestAction, actionSpecialFields))
	tmp = strings.ReplaceAll(tmp, "{{responseFields}}", getSdkFieldTpl(unit.Action.Output(), requestAction, actionSpecialFields))
	tmp = strings.ReplaceAll(tmp, "{{specialFields}}", getSpecialFields(actionSpecialFields))
	sdkDocument += tmp

	return nil
}

func getSpecialFields(actionSpecialFields map[string]string) string {
	var specialFieldsTpl string
	for name, fields := range actionSpecialFields {
		if _, ok := specialFields[name]; !ok {
			specialFields[name] = fields
			var tmpTpl = sdkField
			tmpTpl = strings.ReplaceAll(tmpTpl, "{{name}}", name)
			tmpTpl = strings.ReplaceAll(tmpTpl, "{{fields}}", fields)
			specialFieldsTpl += tmpTpl
		}
	}
	return specialFieldsTpl
}

func getRequestAction(name string) string {
	var str string
	arr := strings.Split(name, ".")
	for _, s := range arr {
		str += ucFirst(s)
	}
	return str
}

func ucFirst(name string) string {
	b := []byte(name)
	if b[0] >= 97 && b[0] <= 122 {
		b[0] -= 32
	}
	return string(b)
}

func getOriginType(fieldType string) string {
	lastIndex := strings.LastIndex(fieldType, ".")
	if lastIndex == -1 {
		return strings.ReplaceAll(fieldType, ".", "")
	}
	a := strings.ReplaceAll(fieldType, ".", "")
	lastIndex = strings.LastIndex(a, "]")
	if lastIndex == -1 {
		return ucFirst(a)
	}
	b := []byte(a)
	if b[lastIndex+1] >= 97 && b[lastIndex+1] <= 122 {
		b[lastIndex+1] -= 32
	}
	return string(b)
}

func getSdkFieldTpl(fields map[string]play.ActionField, requestAction string, specialFields map[string]string) string {
	var tmp string
	var names []string
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		field := fields[name]
		fieldType := getOriginType(field.OriginType)

		if field.Child != nil {
			specialFields[fieldType[strings.LastIndex(fieldType, "]")+1:]] = getSdkFieldTpl(field.Child, requestAction, specialFields)
		}
		var keyTag, jsonTag = field.Field, field.Field
		if len(field.Keys) > 0 {
			keyTag = field.Keys[0]
			jsonTag = field.Keys[0]
		}
		tmp += fmt.Sprintf("%s %s `key:\"%s\" json:\"%s\"` \n", ucFirst(strings.TrimPrefix(field.Field, "_")), fieldType, keyTag, jsonTag)
	}
	return tmp
}

func getPackageName(moduleName string) string {
	var name string
	arrs := strings.Split(moduleName, "-")
	for i, data := range arrs {
		if i == 0 {
			name += data
		} else {
			name += ucFirst(data)
		}
	}
	return name
}
