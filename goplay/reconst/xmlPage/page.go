package xmlPage

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/leochen2038/play/codec/protos/golang/json"
	"github.com/leochen2038/play/goplay/env"
)

type Slot struct {
	Name     string `xml:"name,attr"`
	Type     string `xml:"type,attr"`
	Action   string `xml:"action,attr"`
	Redirect string `xml:"redirect,attr"`
	Label    string `xml:"label,attr"`
	Sort     int    `xml:"sort,attr"`
}

type PagesContent struct {
	Module string `xml:"module,attr"`
	Path   string `xml:"path,attr"`
	Pages  []Page `xml:"page"`
}

type Page struct {
	Module         string
	Template       string  `xml:"template,attr"`
	Title          string  `xml:"title,attr"`
	Tip            string  `xml:"tip,attr"`
	Route          string  `xml:"route,attr"`
	Slots          []Slot  `xml:"slots>slot"`
	Content        Content `xml:"content"`
	FieldChangeMap []interface{}
}

type Content struct {
	InputFrom    string    `xml:"inputFrom,attr"`
	OutputFrom   string    `xml:"outputFrom,attr"`
	DataFrom     string    `xml:"dataFrom,attr"`
	BindKeyLimit string    `xml:"bindKeyLimit,attr"`
	Handlers     []Handler `xml:"handler"`
}

type Handler struct {
	InputFrom  string `xml:"inputFrom,attr"`
	OutputFrom string `xml:"outputFrom,attr"`
	DataFrom   string `xml:"dataFrom,attr"`
	Bind       string `xml:"bind,attr"`
}

type StructInfo struct {
	Pkg        string
	StructName string
}

var pageMap = make(map[string]Page)
var projectPath = env.ProjectPath
var projectName = env.ModuleName

func PageStruct() error {
	var err error
	projectPath = env.ProjectPath
	env.ModuleName, err = parseModuleName()
	if err != nil {
		fmt.Println("获取ModuleName错误：", err.Error())
		return err
	}
	projectName = env.ModuleName

	// step1: 解析xml文件
	if err := parsePageXml(); err != nil {
		fmt.Println("解析xml文件错误：", err.Error())
		return err
	}

	if len(pageMap) == 0 {
		fmt.Println("没有解析到xml文件")
		return nil
	}

	// step2: 生成对应的文档
	if err := generatePageDoc(); err != nil {
		fmt.Println("生成对应的文档：", err.Error())
		return err
	}
	fmt.Println("生成页面结构成功")
	return nil

}

// 生成对应的文档
func generatePageDoc() error {
	var data []byte
	var err error

	filePath := env.ProjectPath + "/init.go"
	if data, err = os.ReadFile(filePath); err != nil {
		return err
	}
	src := string(data)
	src += "\n"
	src += "func getPageData() string {\n"
	pageDataStr, err := json.Marshal(pageMap)
	if err != nil {
		return err
	}
	src += "return `" + string(pageDataStr) + "`\n"
	src += "}\n\n"

	//获取页面元素数据
	binds := generateBinds()
	if len(binds) == 0 {
		fmt.Println("没有解析到binds数据")
	}

	pkg, content := generateContent(binds)
	pkg += "\"github.com/leochen2038/play/page\"\n"

	src += "\n\n" + content + "\n\n"

	src += "func initPage(){\n"
	src += "getPageElement()\n"
	src += "page.GeneratePageData(getPageData()) \n"
	src += "}\n"

	src = strings.Replace(src, "\"github.com/leochen2038/play\"", "\"github.com/leochen2038/play\" \n"+pkg, 1)
	src = strings.Replace(src, "setBuildBasePath()", "setBuildBasePath() \n initPage()", 1)

	if err = os.WriteFile(filePath, []byte(src), 0644); err != nil {
		return err
	}
	exec.Command(runtime.GOROOT()+"/bin/gofmt", "-w", filePath).Run()
	return nil
}

// 解析xml文件
func parsePageXml() error {
	fmt.Println("查看并解析 page xml 文件 ...")
	err := filepath.Walk(projectPath+"/assets/page/", func(filename string, fi os.FileInfo, err error) error {
		var page PagesContent
		var data []byte
		if fi != nil && !fi.IsDir() && strings.HasSuffix(filename, ".xml") {
			if data, err = getPathXml(filename); err != nil {
				return err
			}
			decoder := xml.NewDecoder(bytes.NewReader(data))
			if err = decoder.Decode(&page); err != nil {
				return errors.New("check: " + filename + " failure: " + err.Error())
			}
			if len(page.Pages) > 0 {
				for _, onePage := range page.Pages {
					onePage.Module = page.Module
					pageMap[onePage.Route] = onePage
				}
			}
			handleFieldData(data)
		}
		return nil

	})
	return err
}

func getPathXml(path string) (data []byte, err error) {
	var file *os.File
	var regex *regexp.Regexp
	file, err = os.Open(path)
	if err != nil {
		fmt.Println("无法读取XML文件:", err)
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var xmls []string
	var modulePath string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "<module") && strings.Contains(line, "path=\"") {
			regex, err = regexp.Compile("path=\"(.*?)\"")
			if err == nil {
				match := regex.FindStringSubmatch(line)
				if len(match) == 2 {
					modulePath = match[1]
				}
			}
		}
		if modulePath != "" && strings.Contains(line, "route=\"$path") {
			line = strings.Replace(line, "route=\"$path", "route=\""+modulePath, 1)
		}
		if modulePath != "" && strings.Contains(line, "action=\"$path") {
			line = strings.Replace(line, "action=\"$path", "action=\""+modulePath, 1)
		}
		if modulePath != "" && strings.Contains(line, "submitUrl=\"$path") {
			line = strings.Replace(line, "submitUrl=\"$path", "submitUrl=\""+modulePath, 1)
		}
		if modulePath != "" && strings.Contains(line, "redirect=\"$path") {
			line = strings.Replace(line, "redirect=\"$path", "redirect=\""+modulePath, 1)
		}
		if modulePath != "" && strings.Contains(line, "inputFrom=\"$path") {
			line = strings.Replace(line, "inputFrom=\"$path", "inputFrom=\""+modulePath, 1)
		}
		if modulePath != "" && strings.Contains(line, "outputFrom=\"$path") {
			line = strings.Replace(line, "outputFrom=\"$path", "outputFrom=\""+modulePath, 1)
		}
		if modulePath != "" && strings.Contains(line, "dataFrom=\"$path") {
			line = strings.Replace(line, "dataFrom=\"$path", "dataFrom=\""+modulePath, 1)
		}
		xmls = append(xmls, line)

	}
	resContent := strings.Join(xmls, "\n")
	data = []byte(resContent)
	return
}

// 处理field数据
func handleFieldData(data []byte) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var pageKey string
	for {
		t, _ := decoder.Token()
		if t == nil {
			break
		}

		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "page" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "route" {
						pageKey = attr.Value
					}
				}
			}

			if se.Name.Local == "field" {
				//处理field
				var fieldMap = make(map[string]string)
				if pageKey == "" {
					continue
				}
				for _, attr := range se.Attr {
					fieldMap[attr.Name.Local] = attr.Value
				}
				if _, ok := pageMap[pageKey]; ok {
					var fieldPage Page
					fieldPage = pageMap[pageKey]
					fieldPage.FieldChangeMap = append(fieldPage.FieldChangeMap, fieldMap)
					pageMap[pageKey] = fieldPage
				}
			}

		}
	}

}

// 获取binds 数据
func generateBinds() []StructInfo {
	binds := []StructInfo{}
	for _, onePage := range pageMap {
		for _, oneElement := range onePage.Content.Handlers {
			//获取字段数据
			pathBind := strings.ReplaceAll(oneElement.Bind, ".", "/")
			lastSlash := strings.LastIndex(pathBind, "/")
			if lastSlash >= 0 {
				binds = append(binds, StructInfo{
					Pkg:        pathBind[:lastSlash],
					StructName: pathBind[lastSlash+1:],
				})
			}
		}
	}
	return binds
}

// 生成文件内容
func generateContent(binds []StructInfo) (Pkg, content string) {
	pkgs := []string{}
	structs := []string{}
	var PkgStr, newStructStr string
	for _, oneBind := range binds {
		pkg := strings.Replace(oneBind.Pkg, "/", "", -1) + " \"" + projectName + "/processor/" + oneBind.Pkg + "\""
		if !strArrContains(pkgs, pkg) {
			pkgs = append(pkgs, pkg)
		}
		structOne := generatePkg(strings.Split(oneBind.Pkg, "/")) + "." + oneBind.StructName + "{}"
		if !strArrContains(structs, structOne) {
			structs = append(structs, structOne)
		}
	}

	for _, oneStruct := range structs {
		key := strings.TrimRight(strings.Replace(oneStruct, ".", "", -1), "{}")
		newStructStr += "\t page.ActionStructMap[\"" + key + "\"] = &" + oneStruct + "\n"
	}

	PkgStr += "\n"
	PkgStr += "\"fmt\"\n"
	PkgStr += "\"reflect\"\n"

	return PkgStr, fmt.Sprintf(`

func getPageElement() {

	%s
	for k, v := range page.ActionStructMap {
		pValue := reflect.ValueOf(v).Elem().FieldByName("Input")
		resData := page.ParserField1(pValue)
		page.ActionDescMap[k + "#Input"] = resData
		pValueOut := reflect.ValueOf(v).Elem().FieldByName("Output")
		resDataOut := page.ParserField1(pValueOut)
		page.ActionDescMap[k + "#Output"] = resDataOut
	}

	if len(page.ActionDescMap) == 0 {
		fmt.Println("ActionDescMap is empty")
	}
}

`, newStructStr)
}

// 把数组生成 pkg
func generatePkg(pkgArr []string) string {
	if len(pkgArr) == 0 {
		return ""
	}
	if len(pkgArr) == 1 {
		return pkgArr[0]
	}
	pkgStr := pkgArr[0]
	for _, v := range pkgArr[1:] {
		//首字母大写
		pkgStr += strings.ToUpper(string(v[0])) + v[1:]
	}
	return pkgStr
}

// 判断 []string 中是否包含某个字符串
func strArrContains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func parseModuleName() (string, error) {
	modPath := fmt.Sprintf("%s/go.mod", env.ProjectPath)
	_, err := os.Stat(modPath)
	if os.IsNotExist(err) {
		return "", errors.New("can not find go.mod in project")
	}

	file, _ := os.Open(modPath)
	br := bufio.NewReader(file)
	data, _, _ := br.ReadLine()

	return strings.Split(string(data), " ")[1], nil
}
