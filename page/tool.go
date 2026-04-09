package page

import "strings"

// 检查模板
func getTemplate(tem string) *Template {
	for _, v := range templates {
		if v.Name == tem {
			return v
		}
	}
	return nil
}

// 判断 input render
func checkInputRender(key string) bool {
	if strings.Contains(key, "switch") {
		return true
	}
	if strings.Contains(strings.ToLower(key), "upload") {
		return true
	}
	key = strings.Trim(key, "[]")
	for _, v := range inputRenders {
		if v == key {
			return true
		}
	}
	return false
}

// 判断 output render
func checkOutputRender(key string) bool {
	if strings.Contains(key, "switch") {
		return true
	}
	if strings.Contains(strings.ToLower(key), "upload") {
		return true
	}
	for _, v := range outputRenders {
		if v == key {
			return true
		}
	}
	return false
}

// 判断op 的值是否正确
func checkOp(key string) bool {
	for _, v := range options {
		if v == key {
			return true
		}
	}
	return false
}

func checkRenders(key, objType string) bool {
	if objType == "input" {
		return checkInputRender(key)
	}
	return checkOutputRender(key)
}

func checkInputBindKey(key string) bool {
	for _, v := range inputBindKey {
		if v == key {
			return true
		}
	}
	return false
}

func checkStrInArr(data []string, v string) bool {
	for _, iv := range data {
		if iv == v {
			return true
		}
	}
	return false
}

// 去除重复字符串
func RemoveRepeatedElement(arr []string) (newArr []string) {
	newArr = make([]string, 0)
	for i := 0; i < len(arr); i++ {
		repeat := false
		for j := i + 1; j < len(arr); j++ {
			if arr[i] == arr[j] {
				repeat = true
				break
			}
		}
		if !repeat {
			newArr = append(newArr, arr[i])
		}
	}
	return newArr
}
