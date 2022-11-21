package initProject

import (
	"fmt"

	"github.com/leochen2038/play/goplay/env"
)

func getHookTpl() string {
	code := fmt.Sprintf(`
package hook

import (
	"%s"
)
`, env.FrameworkName)
	return code + hookCode()
}

func hookCode() string {
	return `
type ServerHook struct {
}

func NewServerHook() play.IServerHook {
	return &ServerHook{}
}

func (h ServerHook) OnBoot(server play.IServer) {
	// TODO
}

func (h ServerHook) OnShutdown(server play.IServer) {
	// TODO
}

func (h ServerHook) OnConnect(sess *play.Session, err error) {
	// TODO
}

func (h ServerHook) OnClose(sess *play.Session, err error) {
	// TODO
}

func (h ServerHook) OnRequest(ctx *play.Context) (err error) {
	// TODO
	return
}

func (h ServerHook) OnResponse(ctx *play.Context) {
	// TODO
}

func (h ServerHook) OnFinish(ctx *play.Context) {
	// TODO
}	
`
}
