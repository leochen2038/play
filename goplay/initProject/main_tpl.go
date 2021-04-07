package initProject

import (
	"fmt"
	"github.com/leochen2038/play/goplay/reconst/env"
)

func getMainTpl() string {
	code := fmt.Sprintf(`
package main

import (
	"encoding/json"
	"fmt"
	"%s"
	"%s/server"
	"time"
)
`, env.FrameworkName, env.FrameworkName)
	return code + serverCode()
}

func serverCode() string {
	return "\nfunc main() {\n\tgo server.BootHttp(server.HttpConfig{\n\t\tAddress: \":9090\",\n\t\tRender: func(ctx *play.Context, err error) {\n\t\t\tvar response []byte\n\t\t\tif err != nil {\n\t\t\t\tif errCode, ok := err.(*play.ErrorCode); ok {\n\t\t\t\t\tresponse = []byte(fmt.Sprintf(`{\"rc\":%d,\"tm\":%d,\"msg\":\"%s\"}`, errCode.Code(), time.Now().Unix(), errCode.Info()))\n\t\t\t\t} else {\n\t\t\t\t\tresponse = []byte(fmt.Sprintf(`{\"rc\":%d,\"tm\":%d,\"msg\":\"%s\"}`, 0x100, time.Now().Unix(), err.Error()))\n\t\t\t\t}\n\t\t\t} else if ctx != nil {\n\t\t\t\tctx.Output.Set(\"rc\", 0)\n\t\t\t\tctx.Output.Set(\"tm\", time.Now().Unix())\n\t\t\t\tresponse, _ = json.Marshal(ctx.Output.Get(\"\"))\n\t\t\t}\n\n\t\t\tctx.HttpResponse.Header().Set(\"Content-Type\", \"application/json\")\n\t\t\tctx.HttpResponse.Write(response)\n\t\t},\n\t})\n\n\tserver.BootPlaysocket(server.PlaysocketConfig{\n\t\tAddress: \":9091\",\n\t\tRender: func(protocol *server.PlayProtocol, ctx *play.Context, err error) {\n\t\t\tvar response []byte\n\t\t\tif protocol.Responed == 1 {\n\t\t\t\tif err != nil {\n\t\t\t\t\tif errCode, ok := err.(*play.ErrorCode); ok {\n\t\t\t\t\t\tresponse = []byte(fmt.Sprintf(`{\"rc\":%d,\"tm\":%d,\"msg\":\"%s\"}`, errCode.Code(), time.Now().Unix(), errCode.Info()))\n\t\t\t\t\t} else {\n\t\t\t\t\t\tresponse = []byte(fmt.Sprintf(`{\"rc\":%d,\"tm\":%d,\"msg\":\"%s\"}`, 0x100, time.Now().Unix(), err.Error()))\n\t\t\t\t\t}\n\t\t\t\t} else {\n\t\t\t\t\tctx.Output.Set(\"rc\", 0)\n\t\t\t\t\tctx.Output.Set(\"tm\", time.Now().Unix())\n\t\t\t\t\tresponse, _ = json.Marshal(ctx.Output.Get(\"\"))\n\t\t\t\t}\n\t\t\t\tprotocol.ResponseMessage(response)\n\t\t\t}\n\t\t},\n\t})\n}"
}
