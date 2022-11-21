package play

import "context"

type Agent interface {
	Request(ctx context.Context, service string, action string, body []byte) ([]byte, error)
	Marshal(ctx context.Context, service string, action string, i interface{}) ([]byte, error)
	Unmarshal(ctx context.Context, service string, action string, data []byte, i interface{}) error
}
