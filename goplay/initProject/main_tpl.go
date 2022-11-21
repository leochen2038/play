package initProject

import (
	"fmt"

	"github.com/leochen2038/play/goplay/env"
)

func getMainTpl(name string) string {
	code := fmt.Sprintf(`
package main

import (
	"fmt"
	"%s/hook"
	"%s/servers"
)

`, name, env.FrameworkName)

	return code + serverCode()
}

func serverCode() string {
	return `
func main() {
	httpInstance := servers.NewHttpInstance("httpServer", ":8090", hook.NewServerHook(), nil)
	if err := servers.Boot(httpInstance); err != nil {
		fmt.Println(err)
	}
}

`
}
