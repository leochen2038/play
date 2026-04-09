package page

import "sync"

type LayoutMap struct {
	data sync.Map
}

type Layout struct {
	Span   int `json:"span" key:"span" note:"占据的列数"`
	Offset int `json:"offset" key:"offset" note:"左侧的间隔格数"`
	Push   int `json:"push" key:"push" note:"向右移动格数"`
	Pull   int `json:"pull" key:"pull" note:"向左移动格数"`
}

// 写操作
func (sm *LayoutMap) Write(key string, value Layout) {
	sm.data.Store(key, value)
}

// 读操作
func (sm *LayoutMap) Read(key string) Layout {
	vali, ok := sm.data.Load(key)
	if ok && vali != nil {
		return vali.(Layout)
	}
	return Layout{}
}

// 初始化共享空间
var layoutMap = LayoutMap{}

func GetLayoutInstance() *LayoutMap {
	return &layoutMap
}
