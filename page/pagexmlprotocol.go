package page

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/leochen2038/play"
)

var pagesXmlData = make(map[string]Page)
var pageMap = make(map[string]XmlPage)

var pageXmlUrls = sync.Map{}
var mu sync.Mutex
var ActionDescMap = make(map[string]map[string]play.ActionField)
var ActionStructMap = make(map[string]play.Processor)
var pageXmlError error

// 获取兼容版的页面数据
func GetPageData(is play.IServer) (map[string]Page, error) {
	xmlInitData, err := getXmlPages(is)
	if err != nil {
		return nil, err
	}
	return xmlInitData, nil
}

func getXmlPages(is play.IServer) (map[string]Page, error) {
	//todo
	if len(pageMap) == 0 {
		fmt.Println("Xml页面结构数据为空")
		return map[string]Page{}, nil
	}

	for route, page := range pageMap {
		defaultUrl := strings.Replace(route, ".", "/", -1) + ".html"

		if _, ok := pagesXmlData[defaultUrl]; ok {
			continue
		}

		pagesXmlDataOne, err := getParsePage(page, is)
		if err != nil {
			pageXmlError = err
			return map[string]Page{}, err
		}
		mu.Lock()
		pagesXmlData[defaultUrl] = pagesXmlDataOne
		mu.Unlock()

	}

	return pagesXmlData, pageXmlError
}

// 获取页面插槽数据
func GetXmlPageSolts(url string) (slots []Slot, err error) {
	page, ok := pageMap[url]
	if !ok {
		err = errors.New("页面：" + url + " 不存在")
		return
	}
	slots = page.Slots
	return
}

// 获取页面数据
func getParsePage(page XmlPage, is play.IServer) (thisPage Page, err error) {
	thisPage.Title = page.Title
	if err = checkTemplate(page.Template); err != nil {
		return thisPage, errors.New("页面：" + page.Route + " " + err.Error())
	}
	thisPage.Template = page.Template
	thisPage.Tip = page.Tip
	thisPage.Module = page.Module

	//获取op
	thisPage.Op, err = getXmlOperate(page, is)
	if err != nil {
		return
	}
	//获取input
	thisPage.Input, err = getXmlPageField(page, "input", is)
	if err != nil {
		return
	}

	//获取output
	thisPage.Output, err = getXmlPageField(page, "output", is)
	if err != nil {
		return
	}
	return
}

// 检查模板
func checkTemplate(template string) (err error) {
	if template == "" {
		err = errors.New("模板不能为空")
		return
	}
	var flag bool
	for _, v := range templates {
		if v.Name == template {
			flag = true
			break
		}
	}
	if !flag {
		err = errors.New("模板不存在")
	}
	return
}

// 检查 op name & type
func checkOpNameAndType(slot Slot, template string) (err error) {
	names := []string{}
	for _, v := range templates {
		if v.Name == template {
			names = v.Slots
		}
	}
	if !checkStrInArr(names, slot.Name) {
		err = errors.New("slot name=" + slot.Name + " 不合法")
		return
	}
	if !checkStrInArr(options, slot.Type) {
		err = errors.New("slot type=" + slot.Type + " 不合法")
		return
	}
	return
}

// 获取op
func getXmlOperate(page XmlPage, is play.IServer) (Operates, error) {
	ops := make(Operates, 0)
	var err error
	for _, op := range page.Slots {
		var operate Operate
		operate.Label = op.Label
		//判断 name & type 是否合法
		if err = checkOpNameAndType(op, page.Template); err != nil {
			return ops, errors.New("页面：" + page.Route + " " + err.Error())
		}
		operate.Op = op.Type
		operate.Slot = op.Name
		operate.Sort = op.Sort
		operate.Submit, err = getXmlFinUrl(op.Action, op.Type, is)
		if err != nil {
			return ops, err
		}
		if op.Redirect != "" {
			operate.Redirect, err = getXmlFinUrl(op.Redirect, op.Type, is)
			if err != nil {
				return ops, err
			}
			oIndex := strings.Index(operate.Redirect, "?")
			if oIndex > -1 {
				operate.Redirect = operate.Redirect[:oIndex]
			}
		}
		ops = append(ops, operate)
	}
	sort.Sort(ops)
	return ops, nil
}

// 获取入参出参数据协议
func getXmlPageField(p XmlPage, SubType string, is play.IServer) ([]map[string]interface{}, error) {
	res := make([]map[string]interface{}, 0)
	var fields []Field
	var dataFrom string
	var bindKeyLimit bool

	if SubType == "input" {
		fields = p.Input.Fields
		dataFrom = p.Input.DataFrom
		if p.Input.BindKeyLimit == "true" {
			bindKeyLimit = true
		}
	} else if SubType == "output" {
		fields = p.Output.Fields
		dataFrom = p.Output.DataFrom
		if p.Output.BindKeyLimit == "true" {
			bindKeyLimit = true
		}
	}

	if len(fields) == 0 {
		return res, nil
	}

	outputExplainData, err := getParasPageField(fields, NewCommonElementExplain(), SubType, p.Route, dataFrom, bindKeyLimit, is)
	if err != nil {
		return res, err
	}
	res = getXmlChildrenData(outputExplainData.getMapOne())
	return res, nil
}

// 生成页面参数
func getParasPageField(fields []Field, explainData *CommonElementExplain, SubType, defaultUrl, dataFrom string, bindKeyLimit bool, is play.IServer) (*CommonElementExplain, error) {
	for _, field := range fields {
		var explain *CommonElementExplain
		var err error
		var fieldMap = make(map[string]string)
		if field.Key == "" {
			return nil, errors.New("处理器：" + defaultUrl + " " + SubType + " key 不能为空")
		}
		//判断 render 是否合法
		if field.Render == "" {
			continue
		}
		if !checkRenders(field.Render, SubType) {
			return nil, errors.New("处理器：" + defaultUrl + " " + SubType + " " + field.Key + " render 不合法")
		}
		fieldMap = getMapData(field)
		fieldMap, err = dealMapData(fieldMap, SubType, defaultUrl, dataFrom, bindKeyLimit, is)
		if err != nil {
			return nil, err
		}

		explain = NewCommonElementExplain()
		for ik, iv := range fieldMap {
			explain.setAttr(ik, iv)
		}

		if field.Extra != nil {
			if _, ok := field.Extra.(map[string]string); ok {
				for ek, ev := range field.Extra.(map[string]string) {
					explain.setAttr("others", ek+"|"+ev)
				}
			} else if _, ok := field.Extra.(map[string]interface{}); ok {
				for ek, ev := range field.Extra.(map[string]interface{}) {
					explain.setAttr("others", ek+"|"+ev.(string))
				}
			}

		}

		if field.Fields != nil {
			newExplain, err := getParasPageField(field.Fields, explain, SubType, defaultUrl, dataFrom, bindKeyLimit, is)
			if err != nil {
				return nil, err
			}
			explainData.AddChildren(newExplain)
		} else {
			explainData.AddChildren(explain)
		}

	}
	return explainData, nil
}

func getMapData(originData Field) map[string]string {
	resMap := make(map[string]string)
	v := reflect.ValueOf(&originData).Elem()
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		key := t.Field(i).Name
		key = strings.ToLower(string(key[0])) + key[1:]
		if v.Field(i).Kind() == reflect.String && v.Field(i).String() != "" {
			resMap[key] = v.Field(i).String()
		}
		if v.Field(i).Kind() == reflect.Int && v.Field(i).Int() != 0 {
			resMap[key] = strconv.FormatInt(v.Field(i).Int(), 10)
		}
	}
	return resMap
}

// 处理input output 数据
func dealMapData(originMap map[string]string, SubType, defaultUrl, dataFrom string, bindKeyLimit bool, is play.IServer) (map[string]string, error) {
	var err error
	var submitUrl string
	originMap["dataType"] = SubType
	_, ok := originMap["dataFrom"]
	if ok {
		originMap["dataFrom"], err = getXmlFinUrl(originMap["dataFrom"], "", is)
		if err != nil {
			return originMap, errors.New("处理器：" + defaultUrl + " " + SubType + " dataFrom 不合法:" + err.Error())
		}
	}

	if !ok && dataFrom != "" && !bindKeyLimit {
		originMap["dataFrom"], err = getXmlFinUrl(dataFrom, "", is)
		if err != nil {
			return originMap, errors.New("处理器：" + defaultUrl + " " + SubType + " dataFrom 不合法:" + err.Error())
		}
	}

	if !ok && dataFrom != "" && bindKeyLimit && originMap["bindKey"] != "" {
		originMap["dataFrom"], err = getXmlFinUrl(dataFrom, "", is)
		if err != nil {
			return originMap, errors.New("处理器：" + defaultUrl + " " + SubType + " dataFrom 不合法:" + err.Error())
		}
	}

	if SubType == "input" && originMap["dataFrom"] != "" && originMap["bindKey"] == "" {
		originMap["bindKey"] = originMap["key"]
	}

	if SubType == "output" && originMap["dataFrom"] != "" && originMap["bindKey"] == "" {
		originMap["bindKey"] = originMap["key"]
	}

	if originMap["submitUrl"] != "" {
		submitUrl = originMap["submitUrl"]
	} else if originMap["action"] != "" {
		submitUrl = originMap["action"]
	}

	if submitUrl != "" {
		originMap["submitUrl"], err = getXmlFinUrl(submitUrl, "", is)
		if err != nil {
			return nil, errors.New("处理器：" + defaultUrl + " " + SubType + " " + originMap["key"] + " submitUrl 不合法:" + err.Error())
		}
	}
	if originMap["enum"] != "" {
		originMap["enum"], err = getEnumData(originMap["enum"], is)
		if err != nil {
			return nil, errors.New("处理器：" + defaultUrl + " " + SubType + " " + originMap["key"] + err.Error())
		}
	}
	return originMap, nil
}

// 获取 enum 的数据
func getEnumData(data string, is play.IServer) (enumStr string, err error) {
	enumArr := strings.Split(data, "|")
	checkHandleKey := false
	if len(enumArr) > 1 {
		checkHandleKey = true
	}
	for _, v := range enumArr {
		enumParam := strings.Split(v, "#")
		if len(enumParam) == 2 || len(enumParam) == 3 {
			var finUrl string
			if enumParam[0] != "" {
				finUrl, err = getXmlFinUrl(enumParam[0], "", is)
			}
			if err != nil {
				err = errors.New(" enum" + v + " " + err.Error())
				return
			}
			if len(enumParam) == 2 {
				if checkHandleKey {
					err = errors.New(" enum" + v + "缺少handleKey参数")
					return
				}
				enumStr += finUrl + "#" + enumParam[1] + "|"
			} else {
				enumStr += finUrl + "#" + enumParam[1] + "#" + enumParam[2] + "|"
			}
		} else {
			err = errors.New(" enum" + v + "不合法")
			return
		}
	}
	enumStr = strings.TrimRight(enumStr, "|")
	return
}

func getXmlChildrenData(data map[string]interface{}) []map[string]interface{} {
	resData := []map[string]interface{}{}
	if _, ok := data["children"]; ok {
		if v, ok := data["children"].([]map[string]interface{}); ok {
			resData = append(resData, v...)
		}
	}
	return resData
}

/**
 * url 格式化
 * fromType "":字段中的请求,有类型:op请求
 */
func getXmlFinUrl(url, fromType string, is play.IServer) (string, error) {
	if len(url) > 4 && url[:4] == "http" {
		index := strings.Index(url, "?")
		if index >= 0 {
			url = url[:index] + "?" + strings.Replace(url[index+1:], "?", "&", -1)
		}
		return url, nil
	}
	rootUrl, params, _ := parseUrl(url)
	if len(params) == 0 {
		return getFrameUrl(rootUrl, fromType, is)
	}
	resUrl, err := getFrameUrl(rootUrl, fromType, is)
	if err != nil {
		return resUrl, err
	}
	resRootUrl, resParams, ext := parseUrl(resUrl)
	//合并参数
	var resUrlParams string
	for k, v := range resParams {
		if _, ok := params[k]; ok {
			resUrlParams += k + "=" + params[k] + "&"
		} else {
			resUrlParams += k + "=" + v + "&"
		}
	}
	//接口中不存在的参数
	for k, v := range params {
		if _, ok := resParams[k]; !ok {
			resUrlParams += k + "=" + v + "&"
		}
	}

	return resRootUrl + ext + "?" + strings.TrimRight(resUrlParams, "&"), nil
}

// 获取带有框架入参的url
func getFrameUrl(url, fromType string, is play.IServer) (string, error) {
	if urlValue, ok := pageXmlUrls.Load(url); ok && fromType == "" {
		return urlValue.(string), nil
	}

	defer func() {
		if err := recover(); err != nil {
			fmt.Println("程序 panic 错误：", err)
		}
	}()

	resUrl := url
	var p XmlPage
	var ok bool
	var act *play.ActionUnit

	if p, ok = pageMap[strings.Replace(url, "/", ".", -1)]; ok {
		resUrl += ".html"
	}

	if p.Template == "" {
		act = is.LookupActionUnit(strings.Replace(url, "/", ".", -1))
		if act == nil {
			return url, errors.New("不存在：" + url)
		}
	}

	if p.Template != "" && p.Input.DataFrom != "" {
		defaultDataFrom, _, _ := parseUrl(p.Input.DataFrom)
		act = is.LookupActionUnit(strings.Replace(defaultDataFrom, "/", ".", -1))
		if act == nil {
			return url, errors.New("不存在：" + defaultDataFrom)
		}
	}

	if fromType != "" && p.Template == "" && checkStrInArr(pageOptions, fromType) {
		return resUrl, errors.New("页面不存在：" + resUrl)
	}

	//获取input的参数
	params := []string{}
	if act != nil {
		params = getXmlInputParams(act.Action.Input())
	}
	resUrl += "?"
	if len(params) > 0 {
		for _, ik := range params {
			if strings.Contains(resUrl, "?"+ik+"=") || strings.Contains(resUrl, "&"+ik+"=") {
				continue
			}
			resUrl += ik + "={{" + ik + "}}&"
		}
	}
	urlValue := strings.TrimRight(resUrl, "&?")
	pageXmlUrls.Store(url, urlValue)
	return urlValue, nil

}

// 处理url 并获取链接部分和参数部分
func parseUrl(url string) (rootUrl string, params map[string]string, ext string) {
	params = make(map[string]string)
	index := strings.Index(url, "?")
	if index < 0 {
		rootUrl, ext = getFormatUrlAndExt(url)
		return
	}
	rootUrl, ext = getFormatUrlAndExt(url[:index])
	paramsStr := strings.Replace(url[index+1:], "?", "&", -1)
	for _, v := range strings.Split(paramsStr, "&") {
		if strings.Contains(v, "=") {
			kv := strings.Split(v, "=")
			params[kv[0]] = kv[1]
		}
	}
	return
}

// 获取格式化后的路由和后缀
func getFormatUrlAndExt(url string) (retUrl string, ext string) {
	if strings.Contains(url, "/") && len(url) > 5 && (url[len(url)-5:] == ".html" || url[len(url)-5:] == ".json") {
		ext = url[len(url)-5:]
		url = strings.Replace(strings.Replace(url, ".json", "", -1), ".html", "", -1)
	}
	retUrl = url
	if strings.Contains(url, ".") {
		retUrl = strings.Replace(url, ".", "/", -1)
	}
	return
}

// 获取input的值
func getXmlInputParams(fields map[string]play.ActionField) []string {
	var resData = []string{}
	for _, v := range fields {
		key := v.Field
		if _, ok := v.Tags["key"]; ok {
			key = v.Tags["key"]
		}
		resData = append(resData, key)
	}
	sort.Strings(resData)
	return resData
}

//--------------------------------------------------------------------------

// 生成对应的页面数据
func GeneratePageData(xmlDataStr string) error {
	pageOriginMap := make(map[string]OriginPage)
	err := json.Unmarshal([]byte(xmlDataStr), &pageOriginMap)
	if err != nil {
		return errors.New("json.Unmarshal 错误" + err.Error())
	}
	for k, v := range pageOriginMap {
		pageMap[k], err = generateTargetPageData(v)
		if err != nil {
			fmt.Println("generateTargetPageData 错误", err.Error())
			return err
		}
	}
	return nil
}

// 生成对应的目标page 数据
func generateTargetPageData(originPage OriginPage) (targetPage XmlPage, err error) {
	targetPage = XmlPage{
		Module:   originPage.Module,
		Template: originPage.Template,
		Title:    originPage.Title,
		Tip:      originPage.Tip,
		Route:    originPage.Route,
		Slots:    setSort(originPage.Slots),
	}
	var inputContent, outputContent TargetContent
	inputContent.DataFrom = getDataFromUrl(originPage.Content.DataFrom, originPage.Content.InputFrom, originPage.Content.OutputFrom, "input")
	inputContent.BindKeyLimit = originPage.Content.BindKeyLimit
	outputContent.DataFrom = getDataFromUrl(originPage.Content.DataFrom, originPage.Content.InputFrom, originPage.Content.OutputFrom, "output")
	outputContent.BindKeyLimit = originPage.Content.BindKeyLimit
	for _, item := range originPage.Content.Handlers {
		inputFields, err := getTargetFields(item.Bind, originPage.FieldChangeMap, "input", getDataFromUrl(item.DataFrom, item.InputFrom, item.OutputFrom, "input"))
		if err != nil {
			return targetPage, errors.New("page:" + originPage.Route + "的" + item.Bind + "input数据不存在" + err.Error())
		}
		inputContent.Fields = append(inputContent.Fields, inputFields...)
		outputFields, err := getTargetFields(item.Bind, originPage.FieldChangeMap, "output", getDataFromUrl(item.DataFrom, item.InputFrom, item.OutputFrom, "output"))
		if err != nil {
			return targetPage, errors.New("page:" + originPage.Route + "的" + item.Bind + "output数据不存在" + err.Error())
		}
		outputContent.Fields = append(outputContent.Fields, outputFields...)
	}
	targetPage.Input = inputContent
	targetPage.Output = outputContent
	return targetPage, nil
}

// 获取dataFrom的链接
func getDataFromUrl(dataFrom, inputDataFrom, outputDataFrom, dataType string) string {
	resDataFrom := ""
	if dataType == "input" {
		resDataFrom = inputDataFrom
		if resDataFrom == "" {
			resDataFrom = dataFrom
		}
	} else {
		resDataFrom = outputDataFrom
		if resDataFrom == "" {
			resDataFrom = dataFrom
		}
	}
	return resDataFrom
}

// 获取对应的 Fields
func getTargetFields(bind string, fields []interface{}, dataType, dataFrom string) (targetFields []Field, err error) {
	targetFields = []Field{}
	bindStr := generatePkg(strings.Split(bind, "."))
	if dataType == "input" {
		bindStr = bindStr + "#Input"
	} else if dataType == "output" {
		bindStr = bindStr + "#Output"
	}
	resData := make(map[string]play.ActionField)
	if _, ok := ActionDescMap[bindStr]; ok {
		resData = ActionDescMap[bindStr]
	}

	for _, item := range resData {
		if len(item.Tags) == 0 {
			continue
		}
		if _, ok := item.Tags["render"]; !ok {
			continue
		}
		targetFields = append(targetFields, getElementAttr(item, dataFrom))
	}

	if len(fields) == 0 {
		return
	}

	checkFields := []map[string]string{}
	for _, item := range fields {
		if val, ok := item.(map[string]string); ok {
			if val["type"] == dataType && val["key"] != "" {
				checkFields = append(checkFields, val)
			}
		}
		if val, ok := item.(map[string]interface{}); ok {
			if val["type"] == dataType && val["key"] != "" {
				fieldOne := map[string]string{}
				for k, v := range val {
					if k == "action" {
						k = "submitUrl"
					}
					fieldOne[k] = fmt.Sprintf("%v", v)
				}
				checkFields = append(checkFields, fieldOne)
			}
		}
	}
	if len(checkFields) == 0 {
		return
	}

	checkFieldMap := make(map[int]bool)
	for i, item := range checkFields {
		checkFieldMap[i] = false
		for k, vi := range targetFields {
			if getFieldKeyAndCheck(vi, item["key"]) {
				checkFieldMap[i] = true
				targetFields[k] = setFieldOne(vi, item)
			}
		}
	}

	for i, item := range checkFieldMap {
		if !item {
			fieldOne := Field{}
			targetFields = append(targetFields, setEmptyField(fieldOne, checkFields[i]))
		}
	}

	return
}

// 获取元素列表，并判断是否在列表中
func getFieldKeyAndCheck(field Field, key string) bool {
	if field.Key == key {
		return true
	}
	if len(field.Fields) > 0 {
		for _, item := range field.Fields {
			if getFieldKeyAndCheck(item, key) {
				return true
			}
		}
	}
	return false
}

// 元素覆盖
func setFieldOne(target Field, changeField map[string]string) Field {
	res := target
	tv := reflect.ValueOf(&res).Elem()
	tt := tv.Type()
	if res.Key != changeField["key"] {
		if len(res.Fields) > 0 {
			for ci, child := range res.Fields {
				res.Fields[ci] = setFieldOne(child, changeField)
			}
		}
		return res
	}
	for i := 0; i < tv.NumField(); i++ {
		key := tt.Field(i).Name
		key = strings.ToLower(string(key[0])) + key[1:]
		for k, v := range changeField {
			if key == k {
				if tv.Field(i).Kind() == reflect.String {
					tv.Field(i).SetString(v)
					continue
				}
			}
		}
	}
	return res
}

// 空元素赋值
func setEmptyField(target Field, changeField map[string]string) Field {
	res := target
	tv := reflect.ValueOf(&res).Elem()
	tt := tv.Type()
	for i := 0; i < tv.NumField(); i++ {
		key := tt.Field(i).Name
		key = strings.ToLower(string(key[0])) + key[1:]
		for k, v := range changeField {
			if key == k {
				if tv.Field(i).Kind() == reflect.String {
					tv.Field(i).SetString(v)
					continue
				}
			}
		}
	}
	return res
}

// 获取每一个元素的相关属性
func getElementAttr(element play.ActionField, dataFrom string) (targetElement Field) {
	if _, ok := element.Tags["render"]; !ok {
		return
	}
	targetElement = Field{
		KeyType: element.Typ,
		Extra:   make(map[string]string),
	}
	if dataFrom != "" {
		targetElement.DataFrom = dataFrom
	}
	v := reflect.ValueOf(&targetElement).Elem()
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)
		if !fieldValue.CanSet() {
			continue
		}
		key := strings.ToLower(string(field.Name[0])) + field.Name[1:]
		if _, ok := element.Tags[key]; ok {
			if fieldValue.Kind() == reflect.String {
				fieldValue.SetString(element.Tags[key])
			}
		}
		if key == "sort" {
			fieldValue.SetInt(int64(element.Sort))
		}
	}
	others := make(map[string]string)
	for k, vi := range element.Tags {
		if strings.HasPrefix(k, "v.") || strings.HasPrefix(k, "l.") {
			others[k] = vi
		}
	}
	if len(others) > 0 {
		targetElement.Extra = others
	}
	if element.Child != nil {
		for _, vi := range element.Child {
			childElement := getElementAttr(vi, dataFrom)
			if childElement.Key == "" {
				continue
			}
			targetElement.Fields = append(targetElement.Fields, childElement)
		}
	}
	return targetElement
}

func ParserField1(value reflect.Value) map[string]play.ActionField {
	if !value.IsValid() {
		return nil
	}

	fields := make(map[string]play.ActionField)

	for i := 0; i < value.NumField(); i++ {
		field := play.ActionField{}

		structType := value.Type().Field(i)
		structKey := structType.Tag.Get("key")
		structNote := structType.Tag.Get("note")
		structRequire := structType.Tag.Get("required")
		structDefault := structType.Tag.Get("default")
		if len(structKey) > 0 {
			field.Keys = strings.Split(structKey, ",")
		}

		field.Sort = i
		field.Field = structType.Name
		field.Tags = play.TagLookup(string(structType.Tag))
		field.Typ = structType.Type.String()
		field.OriginType = structType.Type.String()
		field.Desc = structNote
		field.Required = structRequire == "true"
		field.Default = structDefault

		switch structType.Type.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64, reflect.String, reflect.Interface, reflect.Bool:

			fields[field.Field] = field
		case reflect.Array, reflect.Slice:
			elemType := reflect.New(structType.Type.Elem())
			elemValue := reflect.Indirect(elemType)

			if elemValue.Type().Kind() == reflect.Struct {
				if strings.Contains(field.OriginType, "[]struct") {
					field.OriginType = "[]" + field.Field
				}
				field.Typ = "[]object"
				field.Child = ParserField1(elemValue)
			}

			if elemValue.Type().Kind() == reflect.Uint8 {
				field.Typ = "[]byte"
			}

			fields[field.Field] = field
		case reflect.Uint8:
			field.Typ = "byte"

			fields[field.Field] = field
		case reflect.Map:
			field.Typ = "map"
			elemType := reflect.New(structType.Type.Elem())
			elemValue := reflect.Indirect(elemType)

			if elemValue.Type().Kind() == reflect.Struct {
				field.Child = ParserField1(elemValue)
			}

			if elemValue.Type().Kind() == reflect.Map {
				lastIndex := strings.LastIndex(field.OriginType, "]")
				field.OriginType = field.OriginType[:lastIndex+1] + "interface {}"
			}

			fields[field.Field] = field
		case reflect.Struct:
			if strings.Contains(field.OriginType, "struct") {
				field.OriginType = field.Field
			}

			field.Child = ParserField1(value.Field(i))
			field.Typ = "object"

			fields[field.Field] = field
		}
	}

	return fields
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

func setSort(data []Slot) (resList []Slot) {
	sortMap := make(map[string]int)
	for i, v := range data {
		if v.Sort == 0 {
			if _, ok := sortMap[v.Name]; !ok {
				sortMap[v.Name] = 1
			} else {
				sortMap[v.Name]++
			}
			data[i].Sort = sortMap[v.Name]
		}
	}
	return data
}
