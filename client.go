package play

type Client interface {
	Call(service string, action string, req interface{}, respond bool) ([]byte, error)
	ParseResponse([]byte, interface{}) error
}
