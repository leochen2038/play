package play

import (
	"context"
	"errors"
)

var ErrQueryEmptyResult = errors.New("empty result in query")

type Condition struct {
	AndOr bool
	Field string
	Con   string
	Val   interface{}
}

type Query struct {
	Name, Module  string
	DBName, Table string
	dbName, table string
	Conditions    []Condition
	Sets          map[string][]interface{}
	Fields        map[string]struct{}
	Order         [][2]string
	Limit         [2]int64
	Group         []string
	Router        string
	Context       context.Context
}

func (q *Query) Init() {
	q.dbName = q.DBName
	q.table = q.Table
}

func (q *Query) GetOrgDBName() string {
	return q.dbName
}

func (q *Query) GetOrgTable() string {
	return q.table
}
