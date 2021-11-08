package meta

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/leochen2038/play/goplay/reconst/env"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

type Meta struct {
	XMLName  xml.Name     `xml:"meta"`
	Module   string       `xml:"module,attr"`
	Name     string       `xml:"name,attr"`
	Tag      string       `xml:"tag,attr"`
	Key      MetaField    `xml:"key"`
	Fields   MetaFields   `xml:"fields"`
	Strategy MetaStrategy `xml:"strategy"`
}

type MetaFields struct {
	List []MetaField `xml:"field"`
}

type MetaField struct {
	Name    string `xml:"name,attr"`
	Alias   string `xml:"alias,attr"`
	Type    string `xml:"type,attr"`
	Note    string `xml:"note,attr"`
	Default string `xml:"default,attr"`
}

type MetaStrategy struct {
	Storage MetaStorage `xml:"storage"`
}

type MetaStorage struct {
	Type     string `xml:"type,attr"`
	Drive    string `xml:"drive,attr"`
	Database string `xml:"database,attr"`
	Table    string `xml:"table,attr"`
	Router   string `xml:"router,attr"`
}

func MetaGenerator() error {
	return filepath.Walk(env.ProjectPath+"/assets/meta", func(filename string, fi os.FileInfo, err error) error {
		var data []byte
		var meta Meta

		if fi != nil && !fi.IsDir() && strings.HasSuffix(filename, ".xml") {
			if data, err = ioutil.ReadFile(filename); err != nil {
				return err
			}
			if err = xml.Unmarshal(data, &meta); err != nil {
				return errors.New("check: " + filename + " failure:" + err.Error())
			}
			if err = writeMeta(meta); err != nil {
				return errors.New("check: " + filename + " failure: " + err.Error())
			}
			fmt.Println("check:", filename, "success")
		}
		return nil
	})
}

func formatLowerName(name string) string {
	return strings.ToLower(strings.Join(strings.Split(name, "_"), ""))
}

func formatUcfirstName(name string) string {
	var split []string
	for _, v := range strings.Split(name, "_") {
		split = append(split, ucfirst(v))
	}
	return strings.Join(split, "")
}

func generateQueryCode(meta Meta) string {
	whereOr := map[string]string{"Where": "true", "Or": "false"}
	con1List := [...]string{"Equal", "NotEqual", "Less", "Greater", "Like"}
	con2List := [...]string{"Between"}
	conslice := [...]string{"In", "NotIn"}

	funcName := formatUcfirstName(meta.Module) + formatUcfirstName(meta.Name)
	src := "package db\n"
	if meta.Strategy.Storage.Type == "mongodb" {
		if meta.Strategy.Storage.Drive == "" || meta.Strategy.Storage.Drive == "default" {
			meta.Strategy.Storage.Drive = "mongodb"
		}
		if meta.Strategy.Storage.Drive == "mongodb" {
			src += fmt.Sprintf(`
import (
	"context"
	"%s"
	"%s/library/metas"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"%s/database/mongodb"
	"time"
)
`, env.FrameworkName, env.ModuleName, env.FrameworkName)
		} else {
			src += fmt.Sprintf(`
import (
	"context"
	"%s"
	"%s/library/metas"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodb "%s"
	"time"
)
`, env.FrameworkName, env.ModuleName, meta.Strategy.Storage.Drive)
			meta.Strategy.Storage.Drive = "mongodb"
		}
	} else {
		if meta.Strategy.Storage.Drive == "default" || meta.Strategy.Storage.Drive == "" {
			meta.Strategy.Storage.Drive = "mysql"
		}
		if meta.Strategy.Storage.Drive == "mysql" {
			src += fmt.Sprintf(`
import (
	"context"
	"%s"
	"%s/library/metas"
	"%s/database/mysql"
)
`, env.FrameworkName, env.ModuleName, env.FrameworkName)
		} else {
			src += fmt.Sprintf(`
import (
	"context"
	"%s"
	"%s/library/metas"
	mysql "%s"
)
`, env.FrameworkName, env.ModuleName, meta.Strategy.Storage.Drive)
			meta.Strategy.Storage.Drive = "mysql"
		}
	}

	//src += genSubObject(meta, funcName)

	arrayFieldList := make(map[string]string, 0)

	src += fmt.Sprintf(`
type query%s struct {
	QueryInfo play.Query
}
`, funcName)

	var initFields string
	for _, field := range meta.Fields.List {
		initFields += fmt.Sprintf(`"%s":{},`, field.Name)
	}
	initFields += fmt.Sprintf(`"%s":{}`, meta.Key.Name)

	src += fmt.Sprintf(`
func %s(c context.Context) *query%s {
	obj := &query%s{}
	obj.QueryInfo.Module = "%s"
	obj.QueryInfo.Name = "%s"
	obj.QueryInfo.DBName = "%s"
	obj.QueryInfo.Table = "%s"
	obj.QueryInfo.Router = "%s"
	obj.QueryInfo.Context = c
	obj.QueryInfo.Sets = map[string][]interface{}{}
	obj.QueryInfo.Fields = map[string]struct{}{%s}
	return obj
}
`, funcName, funcName, funcName, meta.Module, meta.Name, meta.Strategy.Storage.Database, meta.Strategy.Storage.Table, meta.Strategy.Storage.Router, initFields)

	for _, cond := range con1List {
		// generate key
		for where, wherebool := range whereOr {
			src += fmt.Sprintf(`
func (q *query%s)%s%s%s(val interface{}) *query%s {
	q.QueryInfo.Conditions = append(q.QueryInfo.Conditions, play.Condition{AndOr:%s, Field:"%s", Con:"%s", Val:val})
	return q
}
`, funcName, where, formatUcfirstName(meta.Key.Name), cond, funcName, wherebool, meta.Key.Name, cond)
		}

		// generate fields
		for _, vb := range meta.Fields.List {
			for where, wherebool := range whereOr {
				src += fmt.Sprintf(`
func (q *query%s)%s%s%s(val interface{}) *query%s {
	q.QueryInfo.Conditions = append(q.QueryInfo.Conditions, play.Condition{AndOr:%s, Field:"%s", Con:"%s", Val:val})
	return q
}
`, funcName, where, ucfirst(vb.Name), cond, funcName, wherebool, vb.Name, cond)
			}
		}
	}

	for _, cond := range con2List {
		// generate key
		for where, wherebool := range whereOr {
			src += fmt.Sprintf(`
func (q *query%s)%s%s%s(v1 interface{}, v2 interface{}) *query%s {
	q.QueryInfo.Conditions = append(q.QueryInfo.Conditions, play.Condition{AndOr:%s, Field:"%s", Con:"%s", Val:[2]interface{}{v1, v2}})
	return q
}
`, funcName, where, formatUcfirstName(meta.Key.Name), cond, funcName, wherebool, meta.Key.Name, cond)
		}

		// generate fields
		for _, vb := range meta.Fields.List {
			for where, wherebool := range whereOr {
				src += fmt.Sprintf(`
func (q *query%s)%s%s%s(v1 interface{}, v2 interface{}) *query%s {
	q.QueryInfo.Conditions = append(q.QueryInfo.Conditions, play.Condition{AndOr:%s, Field:"%s", Con:"%s", Val:[2]interface{}{v1, v2}})
	return q
}
`, funcName, where, ucfirst(vb.Name), cond, funcName, wherebool, vb.Name, cond)
			}
		}
	}

	for _, cond := range conslice {
		// generate key
		for where, wherebool := range whereOr {
			src += fmt.Sprintf(`
func (q *query%s)%s%s%s(s []interface{}) *query%s {
	q.QueryInfo.Conditions = append(q.QueryInfo.Conditions, play.Condition{AndOr:%s, Field:"%s", Con:"%s", Val:s})
	return q
}
`, funcName, where, formatUcfirstName(meta.Key.Name), cond, funcName, wherebool, meta.Key.Name, cond)
		}

		// generate fields
		for _, vb := range meta.Fields.List {
			for where, wherebool := range whereOr {
				src += fmt.Sprintf(`
func (q *query%s)%s%s%s(s []%s) *query%s {
	q.QueryInfo.Conditions = append(q.QueryInfo.Conditions, play.Condition{AndOr:%s, Field:"%s", Con:"%s", Val:s})
	return q
}
`, funcName, where, ucfirst(vb.Name), cond, getGolangType(vb.Type), funcName, wherebool, vb.Name, cond)
			}
		}
	}

	src += fmt.Sprintf(`
func (q *query%s)OrderBy(key, val string) *query%s {
	q.QueryInfo.Order = append(q.QueryInfo.Order, [2]string{key, val})
	return q
}
`, funcName, funcName)

	src += fmt.Sprintf(`
func (q *query%s)GroupBy(key string) *query%s {
	q.QueryInfo.Group = append(q.QueryInfo.Group, key)
	return q
}
`, funcName, funcName)
	src += fmt.Sprintf(`
func (q *query%s)Count() (int64, error) {
	return %s.Count(&q.QueryInfo)
}
`, funcName, meta.Strategy.Storage.Drive)

	src += fmt.Sprintf(`
func (q *query%s)Delete() (int64, error) {
	return %s.Delete(&q.QueryInfo)
}
`, funcName, meta.Strategy.Storage.Drive)

	src += fmt.Sprintf(`
func (q *query%s)Limit(start int64, count int64) *query%s {
	q.QueryInfo.Limit[0] = start
	q.QueryInfo.Limit[1] = count
	return q
}
`, funcName, funcName)

	if meta.Strategy.Storage.Type == "mongodb" {
		src += fmt.Sprintf(`
func (q *query%s)UpdateAndGetOne() (*metas.%s, error) {
	m := &metas.%s{}
	if err := %s.UpdateAndGetOne(m, &q.QueryInfo); err != nil {
		return nil, err 
	}
	return m, nil
}
`, funcName, funcName, funcName, meta.Strategy.Storage.Drive)
	}

	if meta.Strategy.Storage.Type == "mongodb" {
		src += fmt.Sprintf(`
func (q *query%s)GetOne() (*metas.%s, error) {
	m := &metas.%s{}
	if err := %s.GetOne(m, &q.QueryInfo); err != nil {
		return nil, err 
	}
`, funcName, funcName, funcName, meta.Strategy.Storage.Drive)
		for k, v := range arrayFieldList {
			src += fmt.Sprintf(
				`if metas.%s == nil {
		metas.%s = make(%s, 0)
	}
`, k, k, v)
		}
		src += `
	return m, nil
}
`
	} else {
		src += fmt.Sprintf(`
func (q *query%s)GetOne() (*metas.%s, error) {
	m := &metas.%s{}
	if err := %s.GetOne(m, &q.QueryInfo); err != nil {
		return nil, err 
	}
	return m, nil
}
`, funcName, funcName, funcName, meta.Strategy.Storage.Drive)
	}

	src += fmt.Sprintf(`
func (q *query%s)GetList() ([]metas.%s, error) {
	list := make([]metas.%s, 0)
	err := %s.GetList(&list, &q.QueryInfo)
`, funcName, funcName, funcName, meta.Strategy.Storage.Drive)
	if len(arrayFieldList) > 0 {
		src += fmt.Sprintf(`
	for _, v:= range list {
`)
		for k, v := range arrayFieldList {
			src += fmt.Sprintf(
				`if v.%s == nil {
			v.%s = make(%s, 0)
		}
`, k, k, v)
		}
		src += `}`
	}
	src += `
	return list, err
}`

	if meta.Strategy.Storage.Type == "mongodb" {
		var msrc, csrc string
		if mtimeField, err := getSpTTime(meta.Fields, "mtime"); err == nil {
			msrc = "m." + ucfirst(mtimeField.Name) + " = time.Now().Unix()"
		}
		if ctimeField, err := getSpTTime(meta.Fields, "ctime"); err == nil {
			csrc = "m." + ucfirst(ctimeField.Name) + " = time.Now().Unix()"
		}

		src += fmt.Sprintf(`
func (q *query%s)Save(m *metas.%s) error {
	%s
	if m.Id != primitive.NilObjectID {
		return %s.Save(m, &m.Id, &q.QueryInfo)
	}

	%s
	m.Id = primitive.NewObjectID()
	return %s.Save(m, nil, &q.QueryInfo)
}
`, funcName, funcName, msrc, meta.Strategy.Storage.Drive, csrc, meta.Strategy.Storage.Drive)
	} else {
		if meta.Key.Type == "auto" {
			src += fmt.Sprintf(`
func (q *query%s)Save(m *metas.%s) error {
	id, err := %s.Save(m, &q.QueryInfo)
	m.%s = int(id)
	return err
}
`, funcName, funcName, meta.Strategy.Storage.Drive, formatUcfirstName(meta.Key.Name))
		} else {
			src += fmt.Sprintf(`
func (q *query%s)Save(m *metas.%s) error {
	_, err := %s.Save(m, &q.QueryInfo)
	return err
}
`, funcName, funcName, meta.Strategy.Storage.Drive)
		}
	}

	src += fmt.Sprintf(`
func (q *query%s)Update() (int64, error) {
	return %s.Update(&q.QueryInfo)
}
`, funcName, meta.Strategy.Storage.Drive)

	for _, field := range meta.Fields.List {
		src += fmt.Sprintf(`
func (q *query%s)Set%s(val %s, opt ...string) *query%s {
	args := make([]interface{}, 0, 2)
	if len(opt) > 0 {
		args = append(args, val, opt[0])
	} else {
		args = append(args, val)
	}
	q.QueryInfo.Sets["%s"] = args
	return q
}
`, funcName, formatUcfirstName(field.Name), getGolangType(field.Type), funcName, field.Name)
	}
	return src
}

func getSpTTime(fields MetaFields, t string) (MetaField, error) {
	for _, v := range fields.List {
		if v.Type == t {
			return v, nil
		}
	}
	return MetaField{}, errors.New("can not find " + t)
}

func writeMeta(meta Meta) (err error) {
	var supportDBs = []string{"mysql", "mongodb"}
	var unSupportDB = true
	for _, v := range supportDBs {
		if v == strings.ToLower(meta.Strategy.Storage.Type) {
			unSupportDB = false
			break
		}
	}
	if unSupportDB {
		return errors.New("unSupportDB " + meta.Strategy.Storage.Type)
	}

	if err := os.MkdirAll(env.ProjectPath+"/library/db", 0744); err != nil {
		return err
	}
	filePath := fmt.Sprintf("%s/library/db/%s_%s.go", env.ProjectPath, formatLowerName(meta.Module), formatLowerName(meta.Name))
	src := generateQueryCode(meta)
	if err = ioutil.WriteFile(filePath, []byte(src), 0644); err != nil {
		return
	}
	exec.Command(runtime.GOROOT()+"/bin/gofmt", "-w", filePath).Run()

	if err := os.MkdirAll(env.ProjectPath+"/library/metas", 0744); err != nil {
		return err
	}
	filePath = fmt.Sprintf("%s/library/metas/%s_%s.go", env.ProjectPath, formatLowerName(meta.Module), formatLowerName(meta.Name))
	src = generateMetaCode(meta)
	if err = ioutil.WriteFile(filePath, []byte(src), 0644); err != nil {
		return
	}

	exec.Command(runtime.GOROOT()+"/bin/gofmt", "-w", filePath).Run()
	return
}

func metaDefaultValue(list []MetaField) string {
	var s []string
	for _, field := range list {
		if field.Type == "string" {
			s = append(s, fmt.Sprintf(`%s:"%s"`, ucfirst(field.Name), field.Default))
		} else if field.Type == "int" {
			s = append(s, fmt.Sprintf(`%s:%s`, ucfirst(field.Name), field.Default))
		}
	}
	return strings.Join(s, ", ")
}

func genSubObject(meta Meta, funcName string) (code string) {
	for _, v := range meta.Fields.List {
		if strings.HasPrefix(v.Type, "array:{") && strings.HasSuffix(v.Type, "}") {
			keys := strings.Split(v.Type[7:len(v.Type)-1], ",")
			code = code + fmt.Sprintf("type Meta%s%s struct {\n", funcName, formatUcfirstName(v.Name))
			for _, v := range keys {
				code += "\t" + strings.ReplaceAll(strings.TrimSpace(v), ":", "\t") + "\n"
			}
			code += "}\n"
		}
	}
	return code
}

func getGolangType(t string) string {
	if strings.HasPrefix(t, "array") {
		switch t {
		case "array":
			return "[]interface{}"
		case "array:int":
			return "[]int"
		case "array:int64":
			return "[]int64"
		case "array:string":
			return "[]string"
		case "array:float":
			return "[]float64"
		case "array:array":
			return "[][]interface{}"
		case "array:object":
			return "[]interface{}"
		case "array:map":
			return "[]map[string]interface{}"
		}
	}
	if strings.HasPrefix(t, "map") {
		switch t {
		case "map":
			return "map[string]interface{}"
		case "map:int":
			return "map[string]int"
		case "map:int64":
			return "map[string]int64"
		case "map:string":
			return "map[string]string"
		case "map:map:string":
			return "map[string]map[string]string"
		}
	}
	if t == "ctime" || t == "mtime" || t == "dtime" {
		return "int64"
	}
	if t == "float" {
		return "float64"
	}

	return t
}

func ucfirst(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]
	}
	return ""
}
