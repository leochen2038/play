package meta

import "strconv"

func generateMysqlTable(meta Meta) string {
	tableName := meta.Strategy.Storage.Table
	src := "CREATE TABLE `" + tableName + "` ("
	src += "\n\t`" + meta.Key.Name + "` " + getMysqlType(meta.Key.Type, meta.Key.Length) + " NOT NULL COMMENT '" + meta.Key.Note + "',"
	for _, vb := range meta.Fields.List {
		src += "\n\t`" + vb.Name + "` " + getMysqlType(vb.Type, vb.Length) + " NOT NULL DEFAULT '" + vb.Default + "' COMMENT '" + vb.Note + "',"
	}
	src += "\n\tPRIMARY KEY (`" + meta.Key.Name + "`)\n"
	src += ") ENGINE=InnoDB DEFAULT CHARSET=utf8;"
	return src
}

func getMysqlType(typeName string, length int) string {
	switch typeName {
	case "string":
		if length >= 0 {
			return "VARCHAR(" + strconv.Itoa(length) + ")"
		}
		if length < 0 {
			return "TEXT"
		}
		return "VARCHAR(4096)"
	case "int":
		return "INT(11)"
	case "auto":
		return "INT(11) AUTO_INCREMENT"
	case "int64", "dtime", "ctime", "mtime":
		return "BIGINT(20)"
	case "float64":
		return "DOUBLE(20,2)"
	case "bool":
		return "TINYINT(1)"
	default:
		return "text"
	}
}
