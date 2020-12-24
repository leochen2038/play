package play

type Session interface {
	Logon(ctx *Context) error
	Get(key string) (interface{}, error)
	Set(key string, val interface{})
	GetString(key string) (string, error)
	SetString(key string, val string)
	GetInt(key string) (int error)
	SetInt(key string, val int)
}
