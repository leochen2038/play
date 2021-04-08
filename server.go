package play

import (
	"net"
)

type ServerInstance interface {
	Address() string
	Name() string
	Type() int

	SetOnRequestHandler(handler func(ctx *Context) error)
	SetPackerDelegate(delegate Packer)
	SetRenderHandler(handler func (ctx *Context))

	OnRequest(ctx *Context) error
	Render(ctx *Context)
	Packer() Packer
	Run(net.Listener) error
	Close()
}
