package action

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/leochen2038/play/goplay/env"
)

var registerCode string
var packages = map[string]string{}
var crontab = map[string]struct{}{}

func ReconstAction() (err error) {
	var emptyAction = true
	actions, err := getActions(env.ProjectPath + "/assets/action")
	if err != nil {
		return err
	}
	registerCode = "func init() {\nsetBuildBasePath()\n"
	registerCode += genRegisterCronCode(env.ProjectPath + "/crontab")
	for _, action := range actions {
		emptyAction = false
		metaData := genActionMetaStruct(action.metaData)
		registerCode += "\tplay.RegisterAction(\"" + action.name + "\", " + metaData + ", func()interface{}{return "
		genNextProcessorCode(action.handlerList, &action)
		registerCode = registerCode[:len(registerCode)-1] + "})\n"
	}
	registerCode += "}\n\n"
	registerCode += `func setBuildBasePath() {
	if _, file, _, ok := runtime.Caller(0); ok {
		if strings.LastIndex(file, "/") > 0 {
			play.BuildBasePath = file[:strings.LastIndex(file, "/")+1]
		}
	}
}`

	if err = updateRegister(env.ProjectPath, env.FrameworkName, emptyAction); err != nil {
		return
	}

	// gen caller
	//if err = genCallerCode(actions); err != nil {
	//	return
	//}

	return
}

func genActionMetaStruct(metaData map[string]string) string {
	var metaStruct string = `map[string]string{`
	for k, v := range metaData {
		metaStruct += fmt.Sprintf("\"%s\":\"%s\",", k, v)
	}
	if len(metaData) > 0 {
		metaStruct = metaStruct[:len(metaStruct)-1]
	}
	metaStruct = metaStruct + "}"
	return metaStruct
}

func genRegisterCronCode(path string) (registCode string) {
	reJob := regexp.MustCompile(`type (\w+) struct`)
	rePack := regexp.MustCompile(`package (\w+)`)
	filepath.Walk(path, func(filename string, info os.FileInfo, err error) error {
		if info != nil && !info.IsDir() && len(info.Name()) > 3 && filepath.Ext(info.Name()) == ".go" {
			var packageName string
			code, _ := ioutil.ReadFile(filename)
			submath := reJob.FindAllSubmatch(code, -1)
			if len(submath) > 0 {
				submath := rePack.FindSubmatch(code)
				if len(submath) > 1 {
					packageName = string(submath[1])
					crontab[strings.Replace(filepath.Dir(filename), path, "crontab", 1)] = struct{}{}
				}
			}
			for _, v := range submath {
				fmt.Println("register cronJob", packageName+"."+string(v[1]))
				registCode += fmt.Sprintf("play.RegisterCronJob(\"%s.%s\", func() play.CronJob {return &%s.%s{}})\n", packageName, v[1], packageName, v[1])
			}
		}
		return nil
	})
	return
}

func genNextProcessorCode(proc *processorHandler, act *action) {
	if proc == nil {
		registerCode += "nil"
	} else {
		packageAlias := ""
		name := proc.name
		if err := checkProcessorFile(proc.name); err != nil {
			fmt.Println(err.Error(), "in", act.name)
			os.Exit(1)
		}
		nameSlice := strings.Split(proc.name, ".")

		if len(nameSlice) > 2 {
			packageAlias = nameSlice[0]
			for i := 1; i < len(nameSlice)-1; i++ {
				packageAlias += strings.ToUpper(string(nameSlice[i][0])) + nameSlice[i][1:]
			}
			//packageAlias = strings.Join(nameSlice[:len(nameSlice)-1], "_")
			name = packageAlias + "." + nameSlice[len(nameSlice)-1]
		}
		packages[strings.ReplaceAll(proc.name[:strings.LastIndex(proc.name, ".")], ".", "/")] = packageAlias
		registerCode += "play.NewProcessorWrap(new(" + name + "),"
		registerCode += "func(p play.Processor, ctx *play.Context) (string, error) {return play.RunProcessor(unsafe.Pointer(p.(*" + name + ")), unsafe.Sizeof(*p.(*" + name + ")),p, ctx)},"
		if proc.next == nil {
			registerCode += "nil)"
		} else {
			registerCode += "map[string]*play.ProcessorWrap{"
			for _, v := range proc.next {
				registerCode += "\"" + v.rcstring + "\":"
				genNextProcessorCode(v, act)
			}
			registerCode += "})"
		}
	}
	registerCode += ","
}

func updateRegister(project, frameworkName string, emptyAction bool) (err error) {
	var module string
	if module, err = parseModuleName(project); err != nil {
		return
	}

	src := "package main\n\n"
	if len(crontab) > 0 || len(packages) > 0 || !emptyAction {
		src += "import (\n\t\"" + frameworkName + "\"\n"
	}
	for k, _ := range crontab {
		src += fmt.Sprintf("\t\"%s/%s\"\n", module, k)
	}
	for k, v := range packages {
		src += fmt.Sprintf("\t%s \"%s/processor/%s\"\n", v, module, k)
	}
	src += "\"runtime\"\n"
	src += "\"strings\"\n"
	if len(packages) > 0 {
		src += "\"unsafe\"\n"
	}
	if len(crontab) > 0 || len(packages) > 0 || !emptyAction {
		src += ")\n\n"
	}

	src += registerCode
	path := fmt.Sprintf("%s/init.go", project)
	if err = ioutil.WriteFile(path, []byte(src), 0644); err != nil {
		return
	}

	exec.Command("gofmt", "-w", path).Run()
	return nil
}
