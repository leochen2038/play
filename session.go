package play

type Session interface {
	Logon(ctx *Context) error
	Get(key string) (interface{}, error)
	Set(key string, val interface{})
}
