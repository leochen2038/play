package gendoc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	var service, action = "{{serviceName}}", "{{actionName}}"
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

func genSdk() error {
	_ = os.Mkdir(filepath.Dir(sdkFilePath), 0744)

	// step 1. 打开文件
	var f *os.File
	var err error
	f, err = os.OpenFile(sdkFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	// step 2. 获取内容
	err = play.WalkAction(getSdkActionTpl)
	if err != nil {
		fmt.Println(err)
		return err
	}

	// step 3. 写入文件
	_, err = f.Write([]byte(sdkDocument))
	if err != nil {
		fmt.Println(err)
		return err
	}
	_ = exec.Command("gofmt", "-w", sdkFilePath).Run()

	return nil
}

func getSdkActionTpl(action *play.Action) error {
	var tmp = sdkTemplate
	sdkDocument = strings.ReplaceAll(sdkDocument, "{{packageName}}", getPackageName(sdkFilePath))

	actionSpecialFields := map[string]string{}
	requestAction := getRequestAction(action.Name())
	callAction := "Call" + requestAction

	tmp = strings.ReplaceAll(tmp, "{{requestName}}", "Req"+requestAction)
	tmp = strings.ReplaceAll(tmp, "{{responseName}}", "Res"+requestAction)
	tmp = strings.ReplaceAll(tmp, "{{serviceName}}", getPackageName(sdkFilePath))
	tmp = strings.ReplaceAll(tmp, "{{desc}}", action.MetaData()["desc"])
	tmp = strings.ReplaceAll(tmp, "{{actionName}}", action.Name())
	tmp = strings.ReplaceAll(tmp, "{{callAction}}", callAction)
	tmp = strings.ReplaceAll(tmp, "{{requestFields}}", getSdkFieldTpl(action.Input(), requestAction, actionSpecialFields))
	tmp = strings.ReplaceAll(tmp, "{{responseFields}}", getSdkFieldTpl(action.Output(), requestAction, actionSpecialFields))
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
		tmp += fmt.Sprintf("%s %s `key:\"%s\" json:\"%s\"` \n", ucFirst(strings.TrimPrefix(field.Field, "_")), fieldType, field.Field, field.Field)
	}
	return tmp
}

func getPackageName(sdkFilePath string) string {
	var name string
	filename := strings.TrimSuffix(filepath.Base(sdkFilePath), ".go")
	filename = strings.TrimSuffix(filename, ".sdk")
	arrs := strings.Split(filename, "-")
	for i, data := range arrs {
		if i == 0 {
			name += data
		} else {
			name += ucFirst(data)
		}
	}
	return name
}
