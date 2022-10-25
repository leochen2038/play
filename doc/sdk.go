package doc

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

func {{callAction}}(ctx context.Context, agent play.Agent, req {{requestName}}) (resp {{responseName}}, err error) {
	var service, action = "{{projectName}}", "{{actionName}}"
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
var projectPath string
var projectName string
var sdkFilePath string

func init() {
	projectPath, _ = filepath.Abs(".")
	projectName = getProjectName(filepath.Base(projectPath))

	sdkFilePath = projectPath + "/sdk/" + projectName + "/" + projectName + ".go"

	_ = os.Mkdir(projectPath+"/sdk", 0744)
	_ = os.Mkdir(projectPath+"/sdk/"+projectName, 0744)
}

func DoGenSdkTask() error {

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
	sdkDocument = strings.ReplaceAll(sdkDocument, "{{packageName}}", projectName)

	specialFields := map[string]string{}
	requestAction := getRequestAction(action.Name())
	callAction := "Call" + requestAction + "Action"

	tmp = strings.ReplaceAll(tmp, "{{requestName}}", requestAction+"Request")
	tmp = strings.ReplaceAll(tmp, "{{responseName}}", requestAction+"Response")
	tmp = strings.ReplaceAll(tmp, "{{projectName}}", projectName)
	tmp = strings.ReplaceAll(tmp, "{{actionName}}", action.Name())
	tmp = strings.ReplaceAll(tmp, "{{callAction}}", callAction)
	tmp = strings.ReplaceAll(tmp, "{{requestFields}}", getSdkFieldTpl(action.Input(), requestAction, specialFields))
	tmp = strings.ReplaceAll(tmp, "{{responseFields}}", getSdkFieldTpl(action.Output(), requestAction, specialFields))
	tmp = strings.ReplaceAll(tmp, "{{specialFields}}", getSpecialFields(specialFields))
	sdkDocument += tmp

	return nil
}

func getSpecialFields(specialFields map[string]string) string {
	var specialFieldsTpl string
	for name, fields := range specialFields {
		var specialField = sdkField
		specialField = strings.ReplaceAll(specialField, "{{name}}", name)
		specialField = strings.ReplaceAll(specialField, "{{fields}}", fields)
		specialFieldsTpl += specialField
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
	arr := strings.Split(fieldType, ".")
	if len(arr) == 1 {
		return fieldType
	}
	return arr[len(arr)-1]
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
			fieldType = requestAction + getOriginType(field.OriginType)
			specialFields[fieldType] = getSdkFieldTpl(field.Child, requestAction, specialFields)
		}
		tmp += fmt.Sprintf("%s %s `key:\"%s\" json:\"%s\"` \n", ucFirst(field.Field), fieldType, field.Field, field.Field)
	}
	return tmp
}

func getProjectName(project string) string {
	var name string
	arrs := strings.Split(project, "-")
	for i, data := range arrs {
		if i == 0 {
			name += data
		} else {
			name += ucFirst(data)
		}
	}
	return name
}
