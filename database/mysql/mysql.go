package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/config"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

var dbconnects sync.Map

func getConnect(router string) (*sql.DB, error) {
	var err error
	var dest string
	var dbconnect *sql.DB

	if dest, err = config.String(router); err != nil {
		return nil, fmt.Errorf("can not find mysql config")
	}

	if connect, _ := dbconnects.Load(dest); connect == nil {
		if dbconnect, err = sql.Open("mysql", dest); err != nil {
			return nil, fmt.Errorf("can not find open mysql | %w", err)
		}
		if err = dbconnect.Ping(); err != nil {
			return nil, fmt.Errorf("can not find ping mysql host | %w", err)
		}

		if connect, ok := dbconnects.LoadOrStore(dest, dbconnect); ok {
			_ = dbconnect.Close()
			return connect.(*sql.DB), nil
		}

		dbconnect.SetConnMaxLifetime(100)
		dbconnect.SetMaxIdleConns(10)
		return dbconnect, nil
	} else {
		return connect.(*sql.DB), nil
	}
}

func GetList(dest interface{}, query *play.Query) (err error) {
	var conn *sql.DB
	var rows *sql.Rows
	if conn, err = getConnect(query.Router); err != nil {
		return
	}

	fields := fieldstext(dest)
	where, values := condtext(query)

	rows, err = conn.Query("SELECT "+fields+" FROM "+query.DBName+"."+query.Table+where, values...)

	if err != nil {
		return
	}
	defer rows.Close()

	err = scanAll(rows, dest)
	return
}

func QueryMap(router, sqlStr string, args ...interface{}) (result []map[string]interface{}, err error) {
	var conn *sql.DB
	var rows *sql.Rows
	if conn, err = getConnect(router); err != nil {
		return
	}
	rows, err = conn.Query(sqlStr, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	//获取列名
	columns, _ := rows.Columns()

	//定义一个切片,长度是字段的个数,切片里面的元素类型是sql.RawBytes
	values := make([]sql.RawBytes, len(columns))
	//定义一个切片,元素类型是interface{} 接口
	scanArgs := make([]interface{}, len(values))
	for i := range values {
		//把sql.RawBytes类型的地址存进去了
		scanArgs[i] = &values[i]
	}
	//获取字段值
	for rows.Next() {
		res := make(map[string]interface{})
		rows.Scan(scanArgs...)
		for i, col := range values {
			res[columns[i]] = string(col)
		}
		result = append(result, res)
	}
	return
}

func GetOne(dest interface{}, query *play.Query) (err error) {
	var conn *sql.DB
	var rows *sql.Rows
	if conn, err = getConnect(query.Router); err != nil {
		return
	}

	fields := fieldstext(dest)
	where, values := condtext(query)

	rows, err = conn.Query("SELECT "+fields+" FROM "+query.DBName+"."+query.Table+where, values...)
	if err != nil {
		return
	}
	defer rows.Close()

	err = scanOne(rows, dest)
	return
}

func fieldstext(dest interface{}) string {
	t := reflect.TypeOf(dest)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	numfield := t.NumField()
	fields := make([]string, 0, numfield)

	for i := 0; i < numfield; i++ {
		fields = append(fields, t.Field(i).Tag.Get("db"))
	}

	return strings.Join(fields, ",")
}

func placeholders(v interface{}) string {
	var n int
	switch v.(type) {
	case []interface{}:
		n = len(v.([]interface{}))
	case []int:
		n = len(v.([]int))
	case []int8:
		n = len(v.([]int8))
	case []int16:
		n = len(v.([]int16))
	case []int32:
		n = len(v.([]int32))
	case []int64:
		n = len(v.([]int64))
	case []float32:
		n = len(v.([]float32))
	case []float64:
		n = len(v.([]float64))
	case []string:
		n = len(v.([]string))
	}

	var b strings.Builder
	for i := 0; i < n-1; i++ {
		b.WriteString("?,")
	}
	if n > 0 {
		b.WriteString("?")
	}
	return b.String()
}

func condtext(query *play.Query) (string, []interface{}) {
	values := make([]interface{}, 0, len(query.Conditions))
	fields := make([]string, 0, len(query.Conditions))
	for _, v := range query.Conditions {
		if v.AndOr {
			switch v.Con {
			case "Equal":
				fields = append(fields, v.Field+" = ?")
			case "NotEqual":
				fields = append(fields, v.Field+" != ?")
			case "Less":
				fields = append(fields, v.Field+" < ?")
			case "Greater":
				fields = append(fields, v.Field+" > ?")
			case "Between":
				fields = append(fields, v.Field+" BETWEEN ? AND ?")
			case "In":
				fields = append(fields, v.Field+" IN ("+placeholders(v.Val)+")")
			case "NotIn":
				fields = append(fields, v.Field+" NOT IN ("+placeholders(v.Val)+")")
			case "Like":
				fields = append(fields, v.Field+" LIKE ?")
			}
		}

		if reflect.TypeOf(v.Val).Kind() == reflect.Slice {
			if v.Con == "Between" {
				for _, params := range v.Val.([2]interface{}) {
					values = append(values, params)
				}
			} else {
				switch v.Val.(type) {
				case []interface{}:
					values = append(values, v.Val.([]interface{})...)
				case []int:
					for _, iv := range v.Val.([]int) {
						values = append(values, iv)
					}
				case []int8:
					for _, iv := range v.Val.([]int8) {
						values = append(values, iv)
					}
				case []int16:
					for _, iv := range v.Val.([]int16) {
						values = append(values, iv)
					}
				case []int32:
					for _, iv := range v.Val.([]int32) {
						values = append(values, iv)
					}
				case []int64:
					for _, iv := range v.Val.([]int64) {
						values = append(values, iv)
					}
				case []float32:
					for _, iv := range v.Val.([]float32) {
						values = append(values, iv)
					}
				case []float64:
					for _, iv := range v.Val.([]float64) {
						values = append(values, iv)
					}
				case []string:
					for _, iv := range v.Val.([]string) {
						values = append(values, iv)
					}
				}
			}
		} else {
			values = append(values, v.Val)
		}
	}

	sql := ""
	if len(fields) > 0 {
		sql += " WHERE " + strings.Join(fields, " AND ")
	}

	if len(query.Group) > 0 {
		sql += " GROUP BY " + strings.Join(query.Group, ", ")
	}

	if len(query.Order) > 0 {
		orders := make([]string, 0, len(query.Order))
		for _, v := range query.Order {
			orders = append(orders, v[0]+" "+v[1])
		}
		sql += " ORDER BY " + strings.Join(orders, ", ")
	}

	if query.Limit[0] != 0 || query.Limit[1] != 0 {
		sql += " LIMIT " + strconv.FormatInt(query.Limit[0], 10) + "," + strconv.FormatInt(query.Limit[1], 10)
	}

	return sql, values
}

func scanAll(rows *sql.Rows, i interface{}) error {
	var v, vp reflect.Value

	rValue := reflect.ValueOf(i)
	if rValue.Kind() != reflect.Ptr {
		return errors.New("must pass a pointer, not a value, to StructScan destination")
	}
	if rValue.IsNil() {
		return errors.New("nil pointer passed to StructScan destination")
	}

	direct := reflect.Indirect(rValue)
	slice, err := baseType(rValue.Type(), reflect.Slice)
	if err != nil {
		return err
	}

	isPtr := slice.Elem().Kind() == reflect.Ptr
	base := deref(slice.Elem())
	colums, err := rows.Columns()
	if err != nil {
		return err
	}

	fields, err := traversalsByName(base, colums)
	if err != nil {
		return err
	}

	values := make([]interface{}, len(colums))

	for rows.Next() {
		vp = reflect.New(base)
		v = reflect.Indirect(vp)

		err = fieldsByTraversal(v, fields, values)
		if err != nil {
			return err
		}

		err = rows.Scan(values...)

		if err != nil {
			return err
		}

		if isPtr {
			direct.Set(reflect.Append(direct, vp))
		} else {
			direct.Set(reflect.Append(direct, v))
		}
	}

	return nil
}

func scanOne(rows *sql.Rows, i interface{}) error {
	var vp reflect.Value
	var empty = true
	rValue := reflect.ValueOf(i)
	if rValue.Kind() != reflect.Ptr {
		return errors.New("must pass a pointer, not a value, to StructScan destination")
	}
	if rValue.IsNil() {
		return errors.New("nil pointer passed to StructScan destination")
	}

	direct := reflect.Indirect(rValue)
	base := deref(rValue.Type())
	colums, err := rows.Columns()
	if err != nil {
		return err
	}

	fields, err := traversalsByName(base, colums)
	if err != nil {
		return err
	}

	values := make([]interface{}, len(colums))

	if rows.Next() {
		empty = false
		vp = reflect.New(base)

		err = fieldsByTraversal(vp, fields, values)
		if err != nil {
			return err
		}

		err = rows.Scan(values...)
		if err != nil {
			return err
		}

		direct.Set(reflect.Indirect(vp))
	}

	if empty {
		return play.ErrQueryEmptyResult
	}

	return nil
}

func fieldsByTraversal(v reflect.Value, traversals []int, values []interface{}) error {
	v = reflect.Indirect(v)
	if v.Kind() != reflect.Struct {
		return errors.New("argument not a struct")
	}

	for i, traversal := range traversals {
		vv := reflect.Indirect(v).Field(traversal)
		if vv.Kind() == reflect.Ptr && vv.IsNil() {
			alloc := reflect.New(deref(vv.Type()))
			vv.Set(alloc)
		}
		values[i] = vv.Addr().Interface()
	}

	return nil
}

func baseType(t reflect.Type, expected reflect.Kind) (reflect.Type, error) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != expected {
		return nil, errors.New("expected " + expected.String() + " but got " + t.Kind().String())
	}
	return t, nil
}

func deref(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func traversalsByName(t reflect.Type, names []string) ([]int, error) {
	r := make([]int, 0, len(names))
	numfield := t.NumField()
	for _, name := range names {
		find := false
		for i := 0; i < numfield; i++ {
			if t.Field(i).Tag.Get("db") == name {
				r = append(r, i)
				find = true
				break
			}
		}

		if !find {
			return nil, errors.New("missing " + name + " field in xml struct")
		}
	}

	return r, nil
}

func Count(query *play.Query) (count int64, err error) {
	var conn *sql.DB
	var rows *sql.Rows
	if conn, err = getConnect(query.Router); err != nil {
		return
	}

	where, values := condtext(query)
	rows, err = conn.Query("SELECT COUNT(*)  FROM "+query.DBName+"."+query.Table+where, values...)
	if err != nil {
		return
	}
	defer rows.Close()

	if rows.Next() {
		err = rows.Scan(&count)
	}

	return
}

func Update(query *play.Query) (modcount int64, err error) {
	var conn *sql.DB
	var res sql.Result
	if conn, err = getConnect(query.Router); err != nil {
		return
	}

	values := make([]interface{}, 0, len(query.Conditions)+len(query.Sets))

	update, value := updatetext(query)
	for _, v := range value {
		values = append(values, v)
	}

	where, value := condtext(query)
	for _, v := range value {
		values = append(values, v)
	}

	res, err = conn.Exec("UPDATE "+query.DBName+"."+query.Table+update+where, values...)
	if err != nil {
		return
	}

	modcount, err = res.RowsAffected()

	return
}

func updatetext(query *play.Query) (string, []interface{}) {
	values := make([]interface{}, 0, len(query.Sets))
	fields := make([]string, 0, len(query.Sets)+1)
	find := false
	for field, v := range query.Sets {
		fields = append(fields, field+" = ?")
		values = append(values, v[0])
		if field == "Fmtime" {
			find = true
		}
	}

	if query.Fields["Fmtime"] && !find {
		fields = append(fields, "Fmtime = ?")
		values = append(values, time.Now().Unix())
	}

	sql := " SET " + strings.Join(fields, ", ")
	return sql, values
}

func Delete(query *play.Query) (delcount int64, err error) {
	var conn *sql.DB
	var res sql.Result
	if conn, err = getConnect(query.Router); err != nil {
		return
	}

	where, values := condtext(query)
	res, err = conn.Exec("DELETE FROM "+query.DBName+"."+query.Table+where, values...)
	if err != nil {
		return
	}

	delcount, err = res.RowsAffected()

	return
}

func Save(meta interface{}, query *play.Query) (id int64, err error) {
	var conn *sql.DB
	var res sql.Result

	if conn, err = getConnect(query.Router); err != nil {
		return
	}

	insert, values := intotext(meta)

	res, err = conn.Exec("REPLACE INTO "+query.DBName+"."+query.Table+insert, values...)

	if err == nil {
		id, err = res.LastInsertId()
	}
	return
}

func intotext(meta interface{}) (string, []interface{}) {
	t := reflect.TypeOf(meta).Elem()
	v := reflect.ValueOf(meta).Elem()
	numfield := t.NumField()

	fields := make([]string, 0, numfield)
	placeholder := make([]string, 0, numfield)
	values := make([]interface{}, 0, numfield)

	for i := 0; i < numfield; i++ {
		fields = append(fields, t.Field(i).Tag.Get("db"))
		placeholder = append(placeholder, "?")

		if t.Field(i).Tag.Get("db") == "Fctime" && v.Field(i).Int() == 0 {
			values = append(values, time.Now().Unix())
		} else if t.Field(i).Tag.Get("db") == "Fmtime" {
			values = append(values, time.Now().Unix())
		} else if t.Field(i).Tag.Get("db") == "Fid" && v.Field(i).Int() == 0 {
			fields = fields[0 : len(fields)-1]
			placeholder = placeholder[0 : len(placeholder)-1]
		} else {
			values = append(values, v.Field(i).Interface())
		}
	}

	sql := "(" + strings.Join(fields, ", ") + ") VALUES(" + strings.Join(placeholder, ", ") + ")"

	return sql, values
}
