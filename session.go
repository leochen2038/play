package play

type Session interface {
	Logon(ctx *Context) error
}
