package server

import (
	"fmt"
	"github.com/leochen2038/play"
	"log"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
)

type HttpConfig struct {
	Address string
	Render  func(ctx *play.Context, err error)
}

func BootHttp(serverConfig HttpConfig) {
	var err error
	if os.Getenv(envGraceful) != "" {
		if id := getGracefulSocket(2); id > 2 {
			if httpListener, err = net.FileListener(os.NewFile(id, "")); err != nil {
				log.Fatal("[http server] error inheriting socket fd")
				os.Exit(1)
			}
			if err = shouldKillParent(); err != nil {
				log.Println("[http server] failed to close parent:", err)
				os.Exit(1)
			}
		} else {
			log.Fatal("[http server] error socket fd < 3")
			os.Exit(1)
		}
	} else {
		if httpListener, err = net.Listen("tcp", serverConfig.Address); err != nil {
			log.Fatal("[http server] listen error: ", err)
			os.Exit(1)
		}
		log.Println("[http server] listen success on: ", serverConfig.Address)
	}

	setHandle(serverConfig)
	server := http.Server{Addr: serverConfig.Address}
	if server.Serve(httpListener) != nil {
		log.Fatal("[http server] : ", err)
	}
}

func setHandle(serverConfig HttpConfig) {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		wg.Add(1)
		var err error
		defer func() {
			wg.Done()
			if panicInfo := recover(); panicInfo != nil {
				log.Fatal(fmt.Errorf("panic: %v\n%v", panicInfo, string(debug.Stack())))
			}
		}()

		ctx := play.NewContextWithInput(play.NewInput(NewHttpParser(request)))
		ctx.HttpRequest = request
		ctx.HttpResponse = writer

		ctx.SpanId = 0
		ctx.TagId = 0

		ctx.TraceId = getMicroUqid("")
		ctx.Version = 3

		var action = request.URL.Path
		if indexDot := strings.Index(action, "."); indexDot > 0 {
			action = action[:indexDot]
		}
		if action == "/" {
			action = "/index"
		}

		err = play.RunAction(strings.ReplaceAll(action[1:], "/", "."), ctx)
		serverConfig.Render(ctx, err)

		return
	})
}
