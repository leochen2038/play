package page

type Page struct {
	Module   string                   `key:"module" json:"module" note:"模块"`
	Title    string                   `key:"title" json:"title" note:"页面标题"`
	Template string                   `key:"template" json:"template" note:"模板"`
	Tip      string                   `json:"tip" key:"tip" note:"页面备注"`
	Op       Operates                 `key:"op" json:"op" note:"页面操作"`
	Input    []map[string]interface{} `key:"input" json:"input" `
	Output   []map[string]interface{} `key:"output" json:"output" `
	//Tips     []string                 `key:"tips" json:"tips" note:"页面全局搜索的标签"`
}

type Operate struct {
	Op       string `key:"op" json:"op" note:"操作类型"`
	Submit   string `key:"submit" json:"submit" note:"提交地址"`
	Label    string `key:"label" json:"label" note:"操作描述"`
	Slot     string `key:"slot" json:"slot" note:"插槽"`
	Sort     int    `key:"sort" json:"sort" note:"排序"`
	Redirect string `key:"redirect" json:"redirect" note:"重定向地址"`
}

type Field struct {
	Type        string `xml:"type,attr"`
	KeyType     string `xml:"keyType,attr"`
	Key         string `xml:"key,attr"`
	Render      string `xml:"render,attr"`
	Label       string `xml:"label,attr"`
	Action      string `xml:"action,attr"`
	SubmitUrl   string `xml:"submitUrl,attr"`
	Enum        string `xml:"enum,attr"`
	Listen      string `xml:"listen,attr"`
	Default     string `xml:"default,attr"`
	Drop        string `xml:"drop,attr"`
	Validator   string `xml:"validator,attr"`
	Regexp      string `xml:"regexp,attr"`
	Required    string `xml:"required,attr"`
	Layout      string `xml:"layout,attr"`
	Tip         string `xml:"tip,attr"`
	ValueFormat string `xml:"valueFormat,attr"`
	RowMergeKey string `xml:"rowMergeKey,attr"`
	ColMergeKey string `xml:"colMergeKey,attr"`
	Sort        int    `xml:"sort,attr"`
	DataFrom    string `xml:"dataFrom,attr"`
	BindKey     string `xml:"bindKey,attr"`
	Align       string `xml:"align,attr"`
	Confirm     string `xml:"confirm,attr"`
	Lazy        string `xml:"lazy,attr"`
	Extra       interface{}
	ExtraParam  string  `xml:"extraParam,attr"`
	Fields      []Field `xml:"field"` // 嵌套字段
}

type Slot struct {
	Name     string `xml:"name,attr"`
	Type     string `xml:"type,attr"`
	Action   string `xml:"action,attr"`
	Redirect string `xml:"redirect,attr"`
	Label    string `xml:"label,attr"`
	Sort     int    `xml:"sort,attr"`
}

type XmlPage struct {
	Module   string
	Template string `xml:"template,attr"`
	Title    string `xml:"title,attr"`
	Tip      string `xml:"tip,attr"`
	Route    string `xml:"route,attr"`
	Slots    []Slot `xml:"slots>slot"`
	Input    TargetContent
	Output   TargetContent
}

type TargetContent struct {
	DataFrom     string
	BindKeyLimit string
	Fields       []Field
}

type OriginPage struct {
	Module         string
	Template       string
	Title          string
	Tip            string
	Route          string
	Slots          []Slot
	Content        Content
	FieldChangeMap []interface{}
}

type Content struct {
	InputFrom    string
	OutputFrom   string
	DataFrom     string
	BindKeyLimit string
	Handlers     []Handler
}

type Handler struct {
	Bind       string
	InputFrom  string
	OutputFrom string
	DataFrom   string
	Fields     []Field
}

type StructInfo struct {
	Pkg        string
	StructName string
}

// Operate 数组根据sort 从小到大排序
type Operates []Operate

func (p Operates) Len() int           { return len(p) }
func (p Operates) Less(i, j int) bool { return p[i].Sort < p[j].Sort }
func (p Operates) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type Pager struct {
	Index int64 `key:"index" json:"index" note:"当前页码" default:"1"`
	Size  int64 `key:"pageSize" json:"pageSize" note:"每页条数 默认20条" default:"20"`
	Total int64 `key:"total" json:"total" note:"总条数"`
}

type RangeMarker struct {
	Start int64 `json:"start" key:"start" note:"开始"`
	End   int64 `json:"end" key:"end" note:"结束"`
}

type MultiSelect struct {
	HandleKey string `json:"handleKey" key:"handleKey" note:"操作值"`
	Value     int    `json:"value" key:"value" note:"提交的值"`
}

type Template struct {
	Name  string   `json:"name" key:"name" note:"模板名"`
	Slots []string `json:"slots" key:"slots" note:"插槽"`
}

type DataFrom struct {
	InputFrom  string `json:"inputFrom" key:"inputFrom" note:"input 数据来源"`
	OutputFrom string `json:"outputFrom" key:"outputFrom" note:"out 数据来源"`
}

type EnumItem struct {
	Value    interface{} `json:"value" key:"value" note:"前端提交值"`
	Label    string      `json:"label" key:"label" note:"前端展示值"`
	Desc     string      `json:"desc" key:"desc" note:"前端描述"`
	Children interface{} `json:"children" key:"children" note:"子节点"`
}

type Histogram struct {
	LabelAlias  string     `json:"labelAlias" key:"labelAlias" note:"X轴名称"`
	ValueAlias  string     `json:"valueAlias" key:"valueAlias" note:"Y轴名称"`
	ToolTip     bool       `json:"toolTip" key:"toolTip" note:"鼠标上去时是否显示那个框"`
	Legend      bool       `json:"legend" key:"legend" note:"是否显示图例"`
	ValueFormat string     `json:"valueFormat" key:"valueFormat" note:"Y轴格式，percent: (value * 100)%, int: 四舍五入整数，thousand: 每三位加,号"`
	List        []EnumItem `json:"list" key:"list" note:"列表数据"`
}

var templates = []*Template{
	{"list", []string{"search", "head", "table"}},
	{"edit", []string{"action"}},
	{"externalContent", []string{"search"}},
	{"chart", []string{"search"}},
	{"tabs", []string{"tab"}},
	{"login", []string{"head", "action", "verify"}},
	{"tree", []string{"head", "node", "leaf"}},
}

var inputBindKey = []string{
	"auto",
	"limit",
}

var inputRenders = []string{
	"date",
	"dateTime",
	"datePicker",
	"dateTimePicker",
	"startDate",
	"endDate",
	"startDateTime",
	"endDateTime",
	"rangeInt",
	"rangeTime",
	"rangeDate",
	"rangeDateTime",
	"int",
	"float",
	"string",
	"radio",
	"checkbox",
	"select",
	"selectSort",
	"switch",
	"uploadImg",
	"uploadFile",
	"uploadBigFile",
	"readOnly",
	"hidden",
	"pager",
	"struct",
	"inline",
	"color",
	"textarea",
	"inputArea",
	"radioImage",
	"editorX",
	"monthPicker",
	"tabs",
	"treePicker",
}

var outputRenders = []string{
	"date",
	"dateTime",
	"text",
	"switch",
	"block",
	"iframe",
	"table",
	"vTable",
	"avatar",
	"image",
	"arrText",
	"tag",
	"prompt",
	"histogram",
	"lineChart",
	"tree",
}

var options = []string{
	"query",
	"submit",
	"confirm",
	"popup",
	"open",
	"export",
}

// option 打开需要页面结构的链接
var pageOptions = []string{
	"popup",
	"open",
}

var valueFormats = []string{
	"second",
	"millisecond",
	"microsecond",
	"nanosecond",
}

var valueAligns = []string{
	"left",
	"right",
	"center",
}
