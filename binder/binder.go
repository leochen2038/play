package binder

import (
	"errors"
	"github.com/tidwall/gjson"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	TimeZone = "Local"
)

func parseSliceKey(k string, c string) (string, error) {
	var kl, cl = len(k), len(c)
	if kl <= cl {
		return "", errors.New(c + " is slice expected with []")
	}
	if k[cl] != '[' {
		return "", errors.New(c + " is slice expected '[' but got '" + string(k[cl]) + "'")
	}
	for i := cl + 1; i < kl; i++ {
		if k[i] == ']' {
			return k[:i+1], nil
		}
		if !(k[i] >= 48 && k[i] <= 57) {
			return "", errors.New(c + " is slice expected 0-9 but got " + string(k[i]))
		}
	}
	return "", errors.New(c + " is slice unknown error")
}

func appendElem(vField reflect.Value, tField reflect.StructField, str string, gValue *gjson.Result) (reflect.Value, error) {
	if regexPattern := tField.Tag.Get("regex"); regexPattern != "" {
		if match, _ := regexp.MatchString(regexPattern, str); match == false {
			return vField, errors.New("value is mismatch")
		}
	}

	if val, err := parse(tField, str, gValue); err != nil {
		return vField, err
	} else {
		vField = reflect.Append(vField, reflect.ValueOf(val))
	}
	return vField, nil
}

func setVal(vField reflect.Value, tField reflect.StructField, str string, gValue *gjson.Result) error {
	if regexPattern := tField.Tag.Get("regex"); regexPattern != "" {
		if match, _ := regexp.MatchString(regexPattern, str); match == false {
			return errors.New("value is mismatch")
		}
	}

	if val, err := parse(tField, str, gValue); err != nil {
		return err
	} else {
		vField.Set(reflect.ValueOf(val))
	}
	return nil
}

func parse(tField reflect.StructField, str string, gValue *gjson.Result) (interface{}, error) {
	switch strings.Trim(tField.Type.String(), "[]") {
	case "interface {}":
		if gValue == nil {
			return str, nil
		}
		return gValue.Value(), nil
	case "string":
		return str, nil
	case "time.Time":
		return parseTime(tField, str)
	case "bool":
		return strconv.ParseBool(str)
	case "byte":
		return parseByte(str, 10)
	case "int":
		return strconv.Atoi(str)
	case "int8":
		return parseInt8(str, 10)
	case "int16":
		return parseInt16(str, 10)
	case "int32":
		return parseInt32(str, 10)
	case "int64":
		return parseInt64(str, 10)
	case "uint":
		return parseUint(str, 10)
	case "uint8":
		return parseUint8(str, 10)
	case "uint16":
		return parseUint16(str, 10)
	case "uint32":
		return parseUint32(str, 10)
	case "uint64":
		return parseUint64(str, 10)
	case "float32":
		return parseFloat32(str)
	case "float64":
		return strconv.ParseFloat(str, 64)
	}
	return nil, errors.New("not supported type " + tField.Type.String())
}

func parseTime(tField reflect.StructField, value string) (t time.Time, err error) {
	var val int64
	if layout := tField.Tag.Get("layout"); layout != "" {
		local, _ := time.LoadLocation(TimeZone)
		return time.ParseInLocation(layout, value, local)
	}
	if val, err = strconv.ParseInt(value, 10, 64); err != nil {
		return
	}
	return time.Unix(val, 0), nil
}

func parseInt8(str string, base int) (int8, error) {
	val, err := strconv.ParseInt(str, base, 8)
	return int8(val), err
}

func parseInt16(str string, base int) (int16, error) {
	val, err := strconv.ParseInt(str, base, 16)
	return int16(val), err
}

func parseInt32(str string, base int) (int32, error) {
	val, err := strconv.ParseInt(str, base, 32)
	return int32(val), err
}

func parseInt64(str string, base int) (int64, error) {
	val, err := strconv.ParseInt(str, base, 64)
	return val, err
}

func parseUint(str string, base int) (uint, error) {
	val, err := strconv.ParseUint(str, base, 0)
	return uint(val), err
}

func parseUint8(str string, base int) (uint8, error) {
	val, err := strconv.ParseUint(str, base, 8)
	return uint8(val), err
}

func parseUint16(str string, base int) (uint16, error) {
	val, err := strconv.ParseUint(str, base, 16)
	return uint16(val), err
}

func parseUint32(str string, base int) (uint32, error) {
	val, err := strconv.ParseUint(str, base, 32)
	return uint32(val), err
}

func parseUint64(str string, base int) (uint64, error) {
	val, err := strconv.ParseUint(str, base, 64)
	return val, err
}

func parseByte(str string, base int) (byte, error) {
	val, err := strconv.ParseInt(str, base, 8)
	return byte(val), err
}

func parseFloat32(str string) (float32, error) {
	val, err := strconv.ParseFloat(str, 32)
	return float32(val), err
}
