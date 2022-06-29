package play

import (
	"fmt"
	"log"
	"reflect"
)

type Node struct {
	Name    string
	Type    interface{}
	Kind    reflect.Kind
	Key     string
	Note    string
	Default string
	Bind    string
	Child   []Node
}

func BuildCaller() {
	for _, act := range actions {
		currentHandler := act.Instance()
		GetInput(currentHandler.p)
		for _, v := range currentHandler.next {
			GetInput(v.p)
		}
	}
}

func ParsePrint(p *Node) {
	if len(p.Child) > 0 {
		for _, v := range p.Child {
			if v.Name == "Input" || v.Name == "Output" {
				fmt.Println(v.Name, ": ", v)
			}
			ParsePrint(&v)
		}
	}
}

func GetInput(p interface{}) (node *Node) {
	var v reflect.Value
	t := reflect.TypeOf(p)
	if t.Kind() == reflect.Ptr {
		v = reflect.ValueOf(p).Elem()
	} else {
		v = reflect.ValueOf(p)
	}

	inputStruct := v.FieldByName("Input")
	inputStruct.Type()
	fieldNum := inputStruct.NumField()

	for i := 0; i < fieldNum; i++ {
		field := inputStruct.Field(i)
		switch field.Kind() {
		case reflect.Struct:
			fmt.Println("find a struct")
			parseStruct(field)
		case reflect.Slice:
			fmt.Println(field.Kind().String())
		default:
			fmt.Println(field.Kind().String())
		}
	}
	return
}

func parseStruct(v reflect.Value) {
	fieldNum := v.NumField()
	for i := 0; i < fieldNum; i++ {
		field := v.Field(i)
		fmt.Println(field.Type().String())
		switch field.Kind() {
		case reflect.Struct:
			fmt.Println("find a struct")
			parseStruct(field)
		default:
			fmt.Println(field.Kind().String())
		}
	}
}

//获取结构体中字段的名称
func GetFieldName2(p interface{}) *Node {
	var tmp Node
	t := reflect.TypeOf(p)

	tmp.Name = t.Name()
	tmp.Type = t.String()
	tmp.Kind = t.Kind()
	switch tmp.Kind {
	case reflect.Ptr:
		el := reflect.New(t.Elem()).Elem()
		ttmp := GetFieldName2(el.Interface())
		if ttmp != nil {
			tmp.Child = append(tmp.Child, *ttmp)
		}
	case reflect.Map:
		el := reflect.New(t.Elem()).Elem()
		ttmp := GetFieldName2(el.Interface())
		if ttmp != nil {
			tmp.Child = append(tmp.Child, *ttmp)
		}
	case reflect.Slice:
		el := reflect.New(t.Elem()).Elem()
		ttmp := GetFieldName2(el.Interface())
		if ttmp != nil {
			tmp.Child = append(tmp.Child, *ttmp)
		}
	case reflect.Struct:
		fieldNum := t.NumField()
		fmt.Println("fieldNum", fieldNum)
		for i := 0; i < fieldNum; i++ {
			var tttmp = Node{}
			tttmp.Name = t.Field(i).Name
			tttmp.Type = t.Field(i).Type.String()
			tttmp.Kind = t.Field(i).Type.Kind()
			tttmp.Note = t.Field(i).Tag.Get("note")
			tttmp.Key = t.Field(i).Tag.Get("key")
			//tttmp.Json = t.Field(i).Tag.Get("json")
			//tttmp.Xml = t.Field(i).Tag.Get("xml")
			tttmp.Bind = t.Field(i).Tag.Get("bind")
			tttmp.Default = t.Field(i).Tag.Get("default")
			k := t.Field(i).Type.Kind()
			switch k {
			case reflect.Map:
				el := reflect.New(t.Field(i).Type.Elem()).Elem()
				ttmp := GetFieldName2(el.Interface())
				if ttmp != nil {
					tttmp.Child = append(tttmp.Child, *ttmp)
				}
			case reflect.Ptr:
				el := reflect.New(t.Field(i).Type.Elem()).Elem()
				ttmp := GetFieldName2(el.Interface())
				if ttmp != nil {
					tttmp.Child = append(tttmp.Child, *ttmp)
				}
			case reflect.Struct:
				ttmp := GetFieldName2(reflect.ValueOf(p).Field(i).Interface())
				if ttmp != nil {
					tttmp.Child = append(tttmp.Child, *ttmp)
				}
			case reflect.Slice:
				el := reflect.New(t.Field(i).Type.Elem()).Elem()
				ttmp := GetFieldName2(el.Interface())
				if ttmp != nil {
					tttmp.Child = append(tttmp.Child, *ttmp)
				}
			default:
				//log.Println("Check type error")
			}
			tmp.Child = append(tmp.Child, tttmp)
		}
	default:
		log.Println("Check type error", tmp.Name)
		return nil
	}

	return &tmp
}
