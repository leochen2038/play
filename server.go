package play

import (
	"net"
)

type ServerInstance interface {
	Address() string
	Name() string
	Type() int
	AppId() int

	SetAppId(int)
	SetOnRequestHandler(handler func(ctx *Context) error)
	SetPackerDelegate(delegate Packer)
	SetResponseHandler(handler func(ctx *Context))

	OnRequest(ctx *Context) error
	Response(ctx *Context)
	Packer() Packer
	Run(net.Listener) error
	Close()
}
