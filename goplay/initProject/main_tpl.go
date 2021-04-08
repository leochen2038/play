package initProject

import (
	"fmt"
	"github.com/leochen2038/play/goplay/reconst/env"
)

func getMainTpl() string {
	code := fmt.Sprintf(`
package main

import "%s/server"
`, env.FrameworkName)
	return code + serverCode()
}

func serverCode() string {
	return `
func main() {
	instance := server.NewHttpInstance("httpServer", ":80", nil, nil)
	if err := server.Boot(instance); err != nil {
		fmt.Println(err)
	}
	server.Wait()
}
`
}
