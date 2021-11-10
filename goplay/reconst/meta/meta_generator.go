package meta

import (
	"fmt"
	"strings"
)

func generateMetaCode(meta Meta) string {
	funcName := formatUcfirstName(meta.Module) + formatUcfirstName(meta.Name)

	src := "package metas\n"
	if meta.Strategy.Storage.Type == "mongodb" {
		src += fmt.Sprintf(`
import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)`)
	}
	src += genSubObject(meta, funcName)
	src += fmt.Sprintf("\ntype %s struct {\n", funcName)
	src += "\t" + formatUcfirstName(meta.Key.Name) + " "
	if meta.Strategy.Storage.Type == "mongodb" {
		src += "primitive.ObjectID\t `bson:\"" + meta.Key.Name + "\""
		if meta.Key.Alias != "" {
			src += ` key:"` + meta.Key.Alias + `"`
		}
		src += "`\n"
	} else if meta.Strategy.Storage.Type == "mysql" {
		if meta.Key.Type == "auto" {
			src += "int\t `db:\"" + meta.Key.Name + "\""
		} else {
			src += meta.Key.Type + "\t `db:\"" + meta.Key.Name + "\""
		}
		if meta.Key.Alias != "" {
			src += ` key:"` + meta.Key.Alias + `"`
		}
		src += "`\n"
	}

	arrayFieldList := make(map[string]string, 0)
	for _, vb := range meta.Fields.List {
		goType := getGolangType(vb.Type)
		src += "\t" + ucfirst(vb.Name) + " " + goType
		if meta.Strategy.Storage.Type == "mongodb" {
			src += "\t `bson:\"" + vb.Name + "\""
			if vb.Alias != "" {
				src += ` key:"` + vb.Alias + `" json:"` + vb.Alias + `"`
			}
			src += "`\n"
		} else {
			src += "\t `db:\"" + vb.Name + "\""
			if vb.Alias != "" {
				src += ` key:"` + vb.Alias + `" json:"` + vb.Alias + `"`
			}
			src += "`\n"
		}

		if strings.HasPrefix(goType, "[]") {
			arrayFieldList[vb.Name] = goType
		}
	}
	src += "}\n"

	src += fmt.Sprintf(`
func New%s() *%s {
	return &%s{%s}
}
`, funcName, funcName, funcName, metaDefaultValue(meta.Fields.List))
	src += "\n"

	if meta.Key.Type != "auto" {
		src += fmt.Sprintf(`func (m *%s)Set%s(val %s) *%s {
	m.%s = val
	return m
}
`, funcName, formatUcfirstName(meta.Key.Name), getGolangType(meta.Key.Type), funcName, formatUcfirstName(meta.Key.Name))
	}

	for _, vb := range meta.Fields.List {
		src += fmt.Sprintf(`func (m *%s)Set%s(val %s) *%s {
	m.%s = val
	return m
}
`, funcName, ucfirst(vb.Name), getGolangType(vb.Type), funcName, ucfirst(vb.Name))
	}
	src += "\n"

	return src
}
