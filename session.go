package play

import (
	"github.com/google/uuid"
)

type Session struct {
	SessId string
	UInfo  interface{}
	Conn   *Conn
	Server IServer
}

func NewSession(c *Conn, server IServer) *Session {
	return &Session{
		Conn:   c,
		SessId: uuid.New().String(),
		Server: server,
	}
}

func (s *Session) Write(res *Response) error {
	if res != nil {
		return s.Server.Transport().Send(s.Conn, res)
	}
	return nil
}
