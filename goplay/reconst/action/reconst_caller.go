package action

import (
	"fmt"
	"github.com/leochen2038/play/goplay/reconst/env"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"unicode"
)

func genCallerCode(actions map[string]action) error {
	src := genHeader()
	for _, act := range actions {
		if act.handlerList == nil {
			continue
		}
		var inputFields = make(map[string]string)
		var outputFields = make(map[string]string)

		proc := act.handlerList
		getFields(proc, inputFields, outputFields)
		for _, v := range proc.next {
			getFields(v, inputFields, outputFields)
		}
		src += genStruct(act.name, inputFields, outputFields)
	}
	return genFile(src)
}

func genFile(src string) (err error) {
	if err = os.MkdirAll(fmt.Sprintf("%s/library/callers", env.ProjectPath), 0744); err != nil {
		return
	}
	filePath := fmt.Sprintf("%s/library/callers/%s.go", env.ProjectPath, strings.ToLower(env.ModuleName))
	if err = ioutil.WriteFile(filePath, []byte(src), 0644); err != nil {
		return
	}
	if err = exec.Command(runtime.GOROOT()+"/bin/gofmt", "-w", filePath).Run(); err != nil {
		fmt.Println(runtime.GOROOT()+"/bin/gofmt", filePath, err)
		return
	}
	return
}

func genHeader() string {
	src := "package callers\n\n"
	src += `import "github.com/leochen2038/play"` + "\n\n"
	return src
}

func genStruct(actionName string, input map[string]string, output map[string]string) string {
	var structName string
	tmp := strings.Split(actionName, ".")
	for _, v := range tmp {
		if env.WithoutModuleName == 0 {
			structName += formatUcfirstName(env.ModuleName + "." + v)
		} else {
			structName += formatUcfirstName(v)
		}
	}

	src := "type " + structName + "Req struct {\n"
	for _, v := range input {
		src += "\t" + v + "\n"
	}
	src += "}\n\n"

	src += "type " + structName + "Resp struct {\n"
	for _, v := range output {
		src += "\t" + v + "\n"
	}
	src += "}\n\n"

	src += "func " + structName + "(c play.Client, req " + structName + "Req, respond bool) (resp *" + structName + "Resp, err error) {\n"
	src += fmt.Sprintf(`	var resByte []byte
	if resByte, err = c.Call("%s", "%s", req, respond); err != nil {
		return nil, err
	}
	if respond {
		err = c.ParseResponse(resByte, resp)
	}
	return`, env.ModuleName, actionName)
	src += "\n}\n\n"
	return src
}

func getFields(handler *processorHandler, input, output map[string]string) {
	filename := fmt.Sprintf("%s/processor/%s.go", env.ProjectPath, strings.ReplaceAll(handler.name, ".", "/"))
	pk := strings.Split(handler.name, ".")
	typeName := pk[len(pk)-1]
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	procStruct := findCode("type "+typeName, data)
	ParseInputOutputByCode(string(procStruct), input, output)
}

func findCode(structName string, data []byte) []byte {
	var start, end, pop int

	re := regexp.MustCompile(`\s+` + structName + ` struct`)
	fi := re.FindIndex(data)

	for i := fi[1]; i < len(data); i++ {
		if data[i] == '{' {
			start = i
			pop++
			break
		}
	}

	for i := start + 1; i < len(data); i++ {
		if data[i] == '{' {
			pop++
		} else if data[i] == '}' {
			pop--
			if pop == 0 {
				end = i
				break
			}
		}
	}

	return data[start : end+1]
}

type Parse struct {
	Tag       int
	Note      int
	CurStruct string
	CurMap    int
	Key       string
	Value     []byte
	Symble    []byte
}

type ParseMap struct {
	Input  map[string]string
	Output map[string]string
}

func ParseInputOutputByCode(code string, input, output map[string]string) {
	var parseMap = map[string]ParseMap{}
	var parse = Parse{}
	parse.Tag = 2
	parse.CurStruct = "default"

	codeArr := strings.Split(code, "\n")
	for _, line := range codeArr {
		if parse.Tag < 2 {
			break
		}
		parseLine(line, &parse, parseMap, input, output)
	}
}

func parseLine(line string, parse *Parse, parseMap map[string]ParseMap, input, output map[string]string) {
	//todo 未考虑注释符在字符串里的场景
	isChange := 0
	if parse.Note == 0 {
		if strings.HasPrefix(line, "//") {
			isChange = 1
			ii := strings.Index(line, "//")
			line = line[0:ii]
		}
		if strings.Contains(line, "/*") {
			isChange = 1
			parse.Note = 1
			be := strings.Index(line, "/*")
			if strings.Contains(line, "*/") {
				parse.Note = 0
				en := strings.Index(line, "*/") + 2
				line = line[0:be] + line[en:]
			} else {
				line = line[0:be]
			}
		}
	} else if parse.Note == 1 {
		if strings.Contains(line, "*/") {
			isChange = 1
			parse.Note = 0
			ii := strings.Index(line, "*/") + 2
			line = line[ii:]
		} else {
			isChange = -1
		}
	}
	if isChange == 1 {
		parseLine(line, parse, parseMap, input, output)
	} else if isChange == 0 {
		switch parse.Tag {
		case 0:
			line = strings.TrimSpace(line)
			if line != "" {
				arr := splitSpace(line)
				if len(arr) > 0 {
					if arr[0] == "type" {
						parse.Tag = 1
						parseLine(line[len(arr[0]):], parse, parseMap, input, output)
					}
				}
			}
			break
		case 1:
			line = strings.TrimSpace(line)
			if line != "" {
				arr := splitSpace(line)
				if len(arr) > 1 && arr[1] == "struct" {
					ii := strings.Index(line, arr[1]) + 6
					parse.CurStruct = arr[0]
					parse.Tag = 2
					parseLine(line[ii:], parse, parseMap, input, output)
				}
			}
			break
		case 2:
			line = strings.TrimSpace(line)
			if line != "" && line[0] == '{' {
				parse.Tag = 3
				parseLine(line[1:], parse, parseMap, input, output)
			}
			break
		case 3:
			line = strings.TrimSpace(line)
			if line != "" {
				arr := splitSpace(line)
				if len(arr) > 1 && arr[1] == "struct" {
					ii := strings.Index(line, arr[1]) + 6
					if arr[0] == "Input" {
						parse.Tag = 4
						parse.CurMap = 1
						if _, ok := parseMap[parse.CurStruct]; !ok {
							parseMap[parse.CurStruct] = ParseMap{Input: map[string]string{}, Output: map[string]string{}}
						}
						parseLine(line[ii:], parse, parseMap, input, output)
					} else if arr[0] == "Output" {
						parse.Tag = 4
						parse.CurMap = 2
						if _, ok := parseMap[parse.CurStruct]; !ok {
							parseMap[parse.CurStruct] = ParseMap{Input: map[string]string{}, Output: map[string]string{}}
						}
						parseLine(line[ii:], parse, parseMap, input, output)
					}
				} else if line == "}" {
					parse.Tag = 0
					parse.CurMap = 0
					parse.CurStruct = ""
				}
			}
			break
		case 4:
			line = strings.TrimSpace(line)
			if line != "" && line[0] == '{' {
				parse.Tag = 5
				parseLine(line[1:], parse, parseMap, input, output)
			}
			break
		case 5:
			line = strings.TrimSpace(line)
			if line != "" {
				arr := splitSpace(line)
				if len(arr) > 1 {
					// 大写字母开头
					if arr[0][0] >= 65 && arr[0][0] <= 90 {
						if parse.CurMap == 1 {
							parse.Tag = 6
							parse.Key = arr[0]
							parse.Value = []byte(arr[0])
							parse.Symble = []byte{}
							parseMap[parse.CurStruct].Input[arr[0]] = ""
							if input != nil {
								input[arr[0]] = ""
							}
							parseLine(line[len(arr[0]):], parse, parseMap, input, output)
						} else if parse.CurMap == 2 {
							parse.Tag = 6
							parse.Key = arr[0]
							parse.Value = []byte(arr[0])
							parse.Symble = []byte{}
							parseMap[parse.CurStruct].Output[arr[0]] = ""
							if output != nil {
								output[arr[0]] = ""
							}
							parseLine(line[len(arr[0]):], parse, parseMap, input, output)
						}
					}
				} else if line == "}" {
					parse.Tag = 3
					parse.CurMap = 0
				}
			}
			break
		case 6:
			if line != "" {
				var index int
				for _, v := range []byte(line) {
					index++
					if len(parse.Symble) == 0 {
						if v >= 65 && v <= 90 {
							parse.Tag = 5
							if parse.CurMap == 1 {
								parseMap[parse.CurStruct].Input[parse.Key] = string(parse.Value)
								if input != nil {
									input[parse.Key] = string(parse.Value)
								}
							} else if parse.CurMap == 2 {
								parseMap[parse.CurStruct].Output[parse.Key] = string(parse.Value)
								if output != nil {
									output[parse.Key] = string(parse.Value)
								}
							}
							parse.CurMap = 0
							parse.Key = ""
							parse.Value = []byte{}
							parseLine(line, parse, parseMap, input, output)
							break
						} else if v == '}' {
							parse.Tag = 3
							if parse.CurMap == 1 {
								parseMap[parse.CurStruct].Input[parse.Key] = string(parse.Value)
								if input != nil {
									input[parse.Key] = string(parse.Value)
								}
							} else if parse.CurMap == 2 {
								parseMap[parse.CurStruct].Output[parse.Key] = string(parse.Value)
								if output != nil {
									output[parse.Key] = string(parse.Value)
								}
							}
							parse.CurMap = 0
							parse.Key = ""
							parse.Value = []byte{}
							break
						} else if v == '`' {
							parse.Tag = 7
							parse.Value = append(parse.Value, v)
							parseLine(line[index:], parse, parseMap, input, output)
							break
						}
					}
					if v == '(' || v == '[' || v == '{' {
						parse.Symble = append(parse.Symble, v)
					} else if len(parse.Symble) > 0 {
						pre := (parse.Symble)[len(parse.Symble)-1]
						if (pre == '(' && v == ')') || (pre == '[' && v == ']') || (pre == '{' && v == '}') {
							if len(parse.Symble) > 1 {
								parse.Symble = (parse.Symble)[0 : len(parse.Symble)-2]
							} else {
								parse.Symble = []byte{}
							}
						}
					}
					parse.Value = append(parse.Value, v)
				}
				if index == len(line) && len(parse.Symble) > 0 {
					parse.Value = append(parse.Value, '\n')
				}
			} else {
				if len(parse.Value) > 0 {
					parse.Value = append(parse.Value, '\n')
				}
			}
			break
		case 7:
			if line != "" {
				var index int
				isEnd := false
				for _, v := range []byte(line) {
					index++
					parse.Value = append(parse.Value, v)
					if v == '`' {
						isEnd = true
						parse.Tag = 5
						if parse.CurMap == 1 {
							parseMap[parse.CurStruct].Input[parse.Key] = string(parse.Value)
							if input != nil {
								input[parse.Key] = string(parse.Value)
							}
						} else if parse.CurMap == 2 {
							parseMap[parse.CurStruct].Output[parse.Key] = string(parse.Value)
							if output != nil {
								output[parse.Key] = string(parse.Value)
							}
						}
						parse.Key = ""
						parse.Value = []byte{}
						parseLine(line[index:], parse, parseMap, input, output)
						break
					}
				}
				if index == len(line) && !isEnd {
					parse.Value = append(parse.Value, '\n')
				}
			} else {
				if len(parse.Value) > 0 {
					parse.Value = append(parse.Value, '\n')
				}
			}
			break
		}
	}
}

func splitSpace(s string) []string {
	var asciiSpaceMap = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}
	var temp = []byte{}
	var arr = []string{}
	for _, v := range []byte(s) {
		if asciiSpaceMap[v] == 1 {
			if len(temp) != 0 {
				arr = append(arr, string(temp))
				temp = []byte{}
			}
			continue
		} else {
			temp = append(temp, v)
		}
	}
	if len(temp) != 0 {
		arr = append(arr, string(temp))
	}
	return arr
}

func formatUcfirstName(name string) string {
	var split []string
	for _, v := range strings.Split(name, ".") {
		split = append(split, ucfirst(v))
	}
	return strings.Join(split, "")
}

func ucfirst(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]
	}
	return ""
}
