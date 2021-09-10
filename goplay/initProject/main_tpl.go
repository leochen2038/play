package initProject

import (
	"fmt"
	"github.com/leochen2038/play/goplay/reconst/env"
)

func getMainTpl(name string) string {
	code := fmt.Sprintf(`
package main

import (
	"embed"
	"fmt"
	"%s/server"
	"%s/transport"
	"%s/hook"
)

`, env.FrameworkName, env.FrameworkName, name)
	return code + serverCode()
}

func serverCode() string {
	return `
func main() {
	serverHook := new(hook.ServerHook)
	httpTransport := transport.NewHttpTransport(1024, embed.FS{}, embed.FS{})
	httpInstance, _ := server.NewHttpInstance("httpServer", ":8090", httpTransport, serverHook)

	if err := server.Boot(httpInstance); err != nil {
		fmt.Println(err)
	}

	server.Wait()
}
`
}
