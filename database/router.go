package database

import (
	"fmt"
	"strings"
)

var host = map[string]struct {
	dest   string
	router Router
}{}

type Router struct {
	Host     string
	Port     string
	Username string
	Password string
	Charset  string
	Database string
	Protocol string
}

func SetRouter(key string, r Router) {
	host[key] = struct {
		dest   string
		router Router
	}{
		dest: fmt.Sprintf("%s:%s@%s(%s:%s)/%s?charset=%s",
			r.Username, r.Password, r.Protocol, r.Host, r.Port, r.Database, r.Charset),
		router: r,
	}
}

func SetDest(key, dest string) {
	host[key] = struct {
		dest   string
		router Router
	}{
		dest:   dest,
		router: decodeHost(dest),
	}
}

func GetRouter(key string) Router {
	if v, ok := host[key]; ok {
		return v.router
	}
	return Router{}
}

func GetDest(key string) string {
	if v, ok := host[key]; ok {
		return v.dest
	}
	return ""
}

func decodeHost(dest string) (r Router) {
	var search int = 0
	prfs := []byte{':', '@', '(', ')', '/', '?', 0}
	var buffer []byte = make([]byte, 0)
	for _, v := range dest {
		if byte(v) != prfs[search] {
			buffer = append(buffer, byte(v))
			continue
		}
		if len(buffer) > 0 {
			switch search {
			case 0:
				r.Username = string(buffer)
			case 1:
				r.Password = string(buffer)
			case 2:
				r.Protocol = string(buffer)
			case 3:
				str := string(buffer)
				if idx := strings.Index(str, ":"); idx >= 0 {
					r.Host, r.Port = str[:idx], str[idx+1:]
				} else {
					r.Host = str
				}
			case 5:
				r.Database = string(buffer)
			}
			buffer = make([]byte, 0)
		}
		if search < len(prfs) {
			search++
		}
	}
	if len(buffer) > 0 {
		if search == 5 {
			r.Database = string(buffer)
		} else {
			args := strings.Split(string(buffer), "&")
			for _, v := range args {
				kv := strings.Split(v, "=")
				if len(kv) == 2 {
					switch kv[0] {
					case "charset":
						r.Charset = kv[1]
					}
				}
			}
		}
	}
	return
}
