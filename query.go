package play

import "errors"

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
	Conditions    []Condition
	Sets          map[string][]interface{}
	Fields        map[string]bool
	Order         [][2]string
	Limit         [2]int64
	Group         []string
	Router        string
}
