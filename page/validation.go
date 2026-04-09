package page

import "sync"

type SharedMap struct {
	data sync.Map
}

type Size struct {
	Height int64 `json:"height" key:"height" note:"长"`
	Width  int64 `json:"width" key:"width" note:"宽"`
}

type Vali struct {
	Len     int64       `json:"len" key:"len" note:"长度必须等于"`
	Min     interface{} `json:"min" key:"min" note:"最小值"`
	Max     int64       `json:"max" key:"max" note:"最大值"`
	Regexp  string      `json:"regexp" key:"regexp" note:"正则表达式"`
	Sizes   Size        `json:"sizes" key:"sizes" note:"长宽"`
	Bytes   int64       `json:"bytes" key:"bytes" note:"字节数"`
	Message string      `json:"message" key:"message" note:"错误信息"`
}

// 写操作
func (sm *SharedMap) Write(key string, value []Vali) {
	sm.data.Store(key, value)
}

// 读操作
func (sm *SharedMap) Read(key string) []Vali {
	vali, ok := sm.data.Load(key)
	if ok && vali != nil {
		return vali.([]Vali)
	}
	return []Vali{}
}

// 初始化共享空间
var sharedMap = SharedMap{}

func GetValInstance() *SharedMap {
	return &sharedMap
}
