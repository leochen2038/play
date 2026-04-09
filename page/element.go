package page

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type CommonElementExplain struct {
	Key         string            `json:"key" note:"提交字段"`
	KeyType     string            `json:"keyType" note:"提交的字段类型"`
	Render      string            `json:"render" note:"渲染类型"`
	Label       string            `json:"label" note:"字段描述"`
	Note        string            `json:"note" note:"字段默认描述"`
	BindKey     string            `json:"bindKey" note:"数据字段"`
	DataFrom    string            `json:"dataFrom" note:"数据请求来源"`
	SubmitUrl   string            `json:"submitUrl" note:"提交地址"`
	Format      string            `json:"format" note:"格式化"`
	DataType    string            `json:"dataType" note:"input/output"`
	Enum        string            `json:"enum" note:"枚举映射"`
	Listen      string            `json:"listen" note:"监听并联动"`
	Required    bool              `json:"required" note:"是否必须"`
	Regexp      string            `json:"regexp" note:"检测的规则 （必须是正则表达式）"`
	Validator   string            `json:"validator" note:"验证字段"`
	Layout      string            `json:"layout" note:"行内布局"`
	Others      map[string]string `json:"others" note:"其他属性"`
	Default     string            `json:"default" note:"默认值"`
	ValueFormat string            `json:"valueFormat" note:"值格式化"`
	RowMergeKey string            `json:"rowMergeKey" note:"rowMergeKey"`
	ColMergeKey string            `json:"colMergeKey" note:"colMergeKey"`
	Tip         string            `json:"tip" note:"备注提醒"`
	Drop        string            `json:"drop" note:"下架"`
	Sort        int               `json:"sort"`
	Align       string            `json:"align" note:"对齐方式"`
	Lazy        string            `json:"lazy" note:"是否懒加载"`
	ExtraParam  string            `json:"extraParam" note:"额外属性"`
	Confirm     bool              `json:"confirm" note:"带确认的开关"`
	Children    Elements          `json:"children"`
}

type Validation struct {
	Regexp string `json:"regexp" note:"验证规则"`
}

type Enum struct {
	Url       string `json:"url" key:"url" note:"枚举域路径"`
	EnumKey   string `json:"enumKey" key:"enumKey" note:"枚举字段"`
	HandleKey string `json:"handleKey" key:"handleKey" note:"操作值"`
}

type Listen struct {
	Key     string        `json:"key" key:"key" note:"监听字段"`
	Reset   bool          `json:"reset" key:"reset" note:"是否重置"`
	Visible []interface{} `json:"visible" key:"visible" note:"显示条件"`
}

type Elements []*CommonElementExplain

func (p Elements) Len() int           { return len(p) }
func (p Elements) Less(i, j int) bool { return p[i].getSort() < p[j].getSort() }
func (p Elements) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func GetMaps(data Elements) []map[string]interface{} {
	var res = []map[string]interface{}{}
	sort.Sort(data)
	for _, v := range data {
		res = append(res, v.getMapOne())
	}
	return res
}

func NewCommonElementExplain() *CommonElementExplain {
	return &CommonElementExplain{}
}

func (f *CommonElementExplain) setAttr(k, v string) *CommonElementExplain {
	fv := reflect.ValueOf(f).Elem()
	ft := fv.Type()
	if k == "render" && v[:2] == "[]" {
		v = v[2:] + "[]"
	}
	if k == "required" && v == "true" {
		f.Required = true
	}
	if k == "confirm" && v == "true" {
		f.Confirm = true
	}
	for i := 0; i < fv.NumField(); i++ {
		key := ft.Field(i).Name
		key = strings.ToLower(string(key[0])) + key[1:]

		if fv.Field(i).Kind() == reflect.String && key == k && k != "others" {
			fv.Field(i).SetString(v)
		}
		if fv.Field(i).Kind() == reflect.Int && key == k && k != "others" {
			vInt, _ := strconv.Atoi(v)
			fv.Field(i).SetInt(int64(vInt))
		}
	}
	if k == "others" {
		arr := strings.Split(v, "|")
		if len(arr) == 2 && strings.Contains(arr[0], ".") {
			if f.Others == nil {
				f.Others = make(map[string]string)
			}
			f.Others[arr[0]] = arr[1]
		}
	}
	return f
}

func (f *CommonElementExplain) AddChildren(el *CommonElementExplain) error {
	renders := append(f.Children, el)
	f.Children = renders
	return nil
}

func (f *CommonElementExplain) getSort() int {
	return f.Sort
}

func (f *CommonElementExplain) getMapOne() map[string]interface{} {
	var oneCommon = make(map[string]interface{})
	originKey := []string{"render", "dataFrom", "tip", "align", "label", "bindKey", "submitUrl", "submitKey"}

	fv := reflect.ValueOf(f).Elem()
	ft := fv.Type()
	for i := 0; i < fv.NumField(); i++ {
		key := ft.Field(i).Name
		key = strings.ToLower(string(key[0])) + key[1:]
		if checkStrInArr(originKey, key) && fv.Field(i).String() != "" {
			vi := fv.Field(i).String()
			if key == "label" && f.DataType == "input" && strings.Trim(fv.Field(i).String(), " ") != "" {
				vi = vi + "："
			}
			if key == "bindkey" && f.DataFrom == "" {
				continue
			}
			oneCommon[key] = vi
		}
	}
	//goto submitKey
	oneCommon = f.getSubmitKey(oneCommon)

	//goto extra
	oneCommon = f.getExtra(oneCommon)

	//goto vali && layout
	oneCommon = f.getValiAndLayout(oneCommon)

	if f.Enum != "" {
		oneCommon["enum"] = getEnum(f.Enum)
	}
	if f.Listen != "" {
		oneCommon["listen"] = getListen(f.Listen)
	}
	thisDefault := f.getDefault()
	if thisDefault != nil {
		oneCommon["default"] = thisDefault
	}
	if len(f.Children) > 0 {
		sort.Sort(f.Children)
		oneCommon["children"] = GetMaps(f.Children)
	}
	return oneCommon
}

// 获取验证规则 和 行内布局
func (f *CommonElementExplain) getValiAndLayout(originData map[string]interface{}) map[string]interface{} {
	if f.DataType != "input" {
		return originData
	}
	ValidationOne, LayoutOne := f.GetOthers()
	// 生成验证规则
	var Validations = []interface{}{}
	if f.Required {
		Validations = append(Validations, Validation{Regexp: "^\\w+$"})
	}
	if f.Regexp != "" {
		Validations = append(Validations, Validation{Regexp: f.Regexp})
	}

	if &ValidationOne != nil {
		valiOne := getValidation(ValidationOne)
		if len(valiOne) > 0 {
			Validations = append(Validations, valiOne)
		}
	}

	if f.Validator != "" {
		valArr := strings.Split(f.Validator, ",")
		for _, v := range valArr {
			validates := sharedMap.Read(strings.Trim(v, " "))
			for _, validate := range validates {
				Validations = append(Validations, getValidation(validate))
			}
		}
	}

	if len(Validations) > 0 {
		originData["validation"] = Validations
	}

	// 生成行内布局
	layoutOne := getLayout(LayoutOne)
	if len(layoutOne) > 0 {
		originData["layout"] = getLayout(LayoutOne)
	} else {
		if f.Layout != "" {
			layoutItem := layoutMap.Read(strings.Trim(f.Layout, " "))
			originData["layout"] = getLayout(layoutItem)
		}
	}

	return originData
}

// 获取extra 中的值
func (f *CommonElementExplain) getExtra(originData map[string]interface{}) map[string]interface{} {
	if f.ValueFormat != "" && checkStrInArr(valueFormats, f.ValueFormat) {
		originData["extra"] = map[string]interface{}{"valueFormat": f.ValueFormat}
	}
	if f.ColMergeKey != "" {
		originData["extra"] = map[string]interface{}{"colMergeKey": strings.Split(f.ColMergeKey, "#")}
	}
	if f.RowMergeKey != "" {
		originData["extra"] = map[string]interface{}{"rowMergeKey": strings.Split(f.RowMergeKey, "#")}
	}
	if f.Align != "" && checkStrInArr(valueAligns, f.Align) {
		originData["extra"] = map[string]interface{}{"align": f.Align}
	}
	if f.Confirm == true {
		originData["extra"] = map[string]interface{}{"confirm": f.Confirm}
	}
	if f.Lazy != "" {
		if f.Lazy == "true" {
			originData["extra"] = map[string]interface{}{"lazy": true}
		} else {
			originData["extra"] = map[string]interface{}{"lazy": false}
		}
	}
	if f.ExtraParam != "" {
		extra := make(map[string]interface{})
		Arr := strings.Split(f.ExtraParam, "|")
		for _, v := range Arr {
			if !strings.Contains(v, ":") {
				continue
			}
			extraArr := strings.Split(v, ":")
			if len(extraArr) != 2 {
				continue
			}
			if extraArr[1] == "true" {
				extra[extraArr[0]] = true
				continue
			}
			if extraArr[1] == "false" {
				extra[extraArr[0]] = false
				continue
			}
			extra[extraArr[0]] = extraArr[1]
		}
		if len(extra) > 0 {
			originData["extra"] = extra
		}
	}
	return originData
}

// 获取submitKey
func (f *CommonElementExplain) getSubmitKey(originData map[string]interface{}) map[string]interface{} {
	if _, ok := originData["submitKey"]; ok {
		return originData
	}

	if f.SubmitUrl != "" {
		originData["submitKey"] = f.Key
	}

	if f.DataType == "input" {
		originData["submitKey"] = f.Key
	}
	return originData
}

// 获取enum 的值
func getEnum(data string) interface{} {
	EnumArr := strings.Split(data, "|")
	Enums := []map[string]string{}
	for _, v := range EnumArr {
		EnumStruct := strings.Split(v, "#")
		var item = make(map[string]string)
		if len(EnumStruct) >= 2 {
			item["enumKey"] = strings.Trim(EnumStruct[1], " ")
			item["url"] = strings.Trim(EnumStruct[0], " ")
			if len(EnumStruct) > 2 {
				item["handleKey"] = strings.Trim(EnumStruct[2], " ")
			}
		}
		Enums = append(Enums, item)
	}
	if len(Enums) == 1 {
		return Enums[0]
	}
	return Enums
}

// 获取listen 的值
func getListen(data string) interface{} {
	ListenArr := strings.Split(data, "|")
	var Listens []Listen
	for _, v := range ListenArr {
		ListenStruct := strings.Split(v, "#")
		var item Listen
		if len(ListenStruct) >= 3 {
			item.Key = strings.Trim(ListenStruct[0], " ")
			if strings.Trim(ListenStruct[1], " ") == "true" {
				item.Reset = true
			}
			if strings.Trim(ListenStruct[2], " ") != "" {
				arrVal := strings.Split(strings.Trim(ListenStruct[2], " "), ",")
				for _, vi := range arrVal {
					val, err := strconv.Atoi(vi)
					if err == nil {
						item.Visible = append(item.Visible, val)
					} else {
						if vi != "" {
							item.Visible = append(item.Visible, vi)
						}
					}
				}
			}
		}
		Listens = append(Listens, item)
	}
	return Listens
}

// 获取default 的值
func (f *CommonElementExplain) getDefault() interface{} {
	if f.Drop == "default" {
		return nil
	}
	if f.Default == "" {
		return nil
	}
	if f.KeyType == "int" {
		thisDefault, _ := strconv.Atoi(f.Default)
		return thisDefault
	}
	if f.KeyType == "int64" {
		thisDefault, _ := strconv.ParseInt(f.Default, 10, 64)
		return thisDefault
	}

	if f.KeyType == "string" {
		return f.Default
	}

	return nil
}

// 获取others 的对象
func (f *CommonElementExplain) GetOthers() (validation Vali, layout Layout) {
	if len(f.Others) > 0 {
		for k, v := range f.Others {
			switch k {
			case "v.len":
				validation.Len, _ = strconv.ParseInt(v, 10, 64)
			case "v.min":
				validation.Min, _ = strconv.ParseInt(v, 10, 64)
			case "v.max":
				validation.Max, _ = strconv.ParseInt(v, 10, 64)
			case "v.bytes":
				validation.Bytes, _ = strconv.ParseInt(v, 10, 64)
			case "v.sizes.width":
				validation.Sizes.Width, _ = strconv.ParseInt(v, 10, 64)
			case "v.sizes.height":
				validation.Sizes.Height, _ = strconv.ParseInt(v, 10, 64)
			case "v.regexp":
				validation.Regexp = v
			case "v.message":
				validation.Message = v
			case "l.span":
				layout.Span, _ = strconv.Atoi(v)
			case "l.offset":
				layout.Offset, _ = strconv.Atoi(v)
			case "l.pull":
				layout.Pull, _ = strconv.Atoi(v)
			case "l.push":
				layout.Push, _ = strconv.Atoi(v)
			}
		}
	}
	return
}

// 获取验证器中的数据
func getValidation(validate Vali) map[string]interface{} {
	var valOne = make(map[string]interface{})
	if validate.Len > 0 {
		valOne["len"] = validate.Len
	}
	if validate.Min != nil {
		valOne["min"] = validate.Min
	}
	if validate.Max > 0 {
		valOne["max"] = validate.Max
	}
	if validate.Regexp != "" {
		valOne["regexp"] = validate.Regexp
	}
	if validate.Message != "" {
		valOne["message"] = validate.Message
	}
	if validate.Sizes.Width > 0 {
		valOne["sizes"] = Size{Width: validate.Sizes.Width, Height: validate.Sizes.Height}
	}
	if validate.Bytes > 0 {
		valOne["bytes"] = validate.Bytes
	}
	return valOne
}

// 获取布局器中的数据
func getLayout(layout Layout) map[string]interface{} {
	var layoutOne = make(map[string]interface{})
	if layout.Span > 0 {
		layoutOne["span"] = layout.Span
	}
	if layout.Offset > 0 {
		layoutOne["offset"] = layout.Offset
	}
	if layout.Pull > 0 {
		layoutOne["pull"] = layout.Pull
	}
	if layout.Push > 0 {
		layoutOne["push"] = layout.Push
	}
	return layoutOne
}
