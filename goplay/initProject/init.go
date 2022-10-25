package initProject

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/leochen2038/play/goplay/env"
)

func InitProject(upgrade bool) (err error) {
	var absPath string
	_, err = os.Stat(env.ProjectPath + "/go.mod")
	if !os.IsNotExist(err) && !upgrade {
		return errors.New("project has alread exist")
	}

	if absPath, err = filepath.Abs(env.ProjectPath); err != nil {
		return
	}

	if err = createMain(filepath.Base(absPath), upgrade); err != nil {
		return
	}
	if err = createAssets(); err != nil {
		return
	}
	if err = createDatabase(); err != nil {
		return
	}
	if err = createLibrary(); err != nil {
		return
	}
	if err = createProcessor(); err != nil {
		return
	}
	if err = createMiddleware(); err != nil {
		return
	}
	if err = createTemplate(); err != nil {
		return
	}
	if err = createHook(); err != nil {
		return
	}
	// if err = createTransport(); err != nil {
	// 	return
	// }
	return
}

func createMain(name string, upgrade bool) (err error) {
	var goVersion = env.GoVersion
	if err = os.MkdirAll(env.ProjectPath, 0744); err != nil {
		return
	}
	if !upgrade {
		if strings.Count(env.GoVersion, ".") > 1 {
			goVersion = env.GoVersion[:strings.LastIndex(env.GoVersion, ".")]
		}
		if err = ioutil.WriteFile(env.ProjectPath+"/main.go", []byte(getMainTpl(name)), 0644); err != nil {
			return
		}
		if err = ioutil.WriteFile(env.ProjectPath+"/go.mod", []byte(fmt.Sprintf(`module %s

go %s

require (
	%s %s
)

`, name, goVersion, env.FrameworkName, env.FrameworkVer)), 0644); err != nil {
			return
		}
		exec.Command("gofmt", "-w", env.ProjectPath+"/main.go").Run()
	}
	if err = ioutil.WriteFile(env.ProjectPath+"/.play", nil, 0644); err != nil {
		return
	}
	return
}

func createAssets() (err error) {
	if err = os.MkdirAll(env.ProjectPath+"/assets/action", 0744); err != nil {
		return
	}
	if err = os.MkdirAll(env.ProjectPath+"/assets/meta", 0744); err != nil {
		return
	}
	return
}

func createLibrary() (err error) {
	return os.Mkdir(env.ProjectPath+"/library", 0744)
}

func createHook() (err error) {
	if err = os.Mkdir(env.ProjectPath+"/hook", 0744); err != nil {
		return
	}
	code := getHookTpl()
	err = ioutil.WriteFile(env.ProjectPath+"/hook/server_hook.go", []byte(code), 0644)
	return
}

// func createTransport() (err error) {
// 	if err = os.Mkdir(env.ProjectPath+"/transport", 0744); err != nil {
// 		return
// 	}
// 	code := getHttpTransportTpl()
// 	err = ioutil.WriteFile(env.ProjectPath+"/transport/http.go", []byte(code), 0644)
// 	return
// }

func createMiddleware() (err error) {
	return os.Mkdir(env.ProjectPath+"/middleware", 0744)
}

func createTemplate() (err error) {
	return os.Mkdir(env.ProjectPath+"/template", 0744)
}

func createDatabase() (err error) {
	return os.Mkdir(env.ProjectPath+"/database", 0744)
}

func createProcessor() (err error) {
	return os.Mkdir(env.ProjectPath+"/processor", 0744)
}

func fmtCode(path string) {
	filepath.Walk(path, func(filename string, info os.FileInfo, err error) error {
		if info.IsDir() && filename[0:1] != "." && filename != path {
			fmtCode(filename)
		}
		if strings.HasSuffix(filename, ".go") {
			exec.Command("gofmt", "-w", filename).Run()
		}
		return nil
	})
}
