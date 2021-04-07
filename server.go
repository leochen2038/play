package play

import (
	"net"
	"time"
)

type ServerInstance interface {
	Address() string
	Name() string
	Type() int
	RequestTimeout() time.Duration
	SetPackerDelegate(delegate Packer)
	Packer() Packer

	OnRequest(ctx *Context) error
	OnResponse(ctx *Context) error
	Run(net.Listener) error
	Close()
}
