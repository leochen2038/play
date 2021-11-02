package play

import (
	"context"
	"github.com/google/uuid"
)

type Session struct {
	SessId    string
	UInfo     interface{}
	Conn      *Conn
	Server    IServer
	ctx       context.Context
	ctxCancel context.CancelFunc
}

func NewSession(cxt context.Context, c *Conn, server IServer) *Session {
	sess := &Session{
		Conn:   c,
		SessId: uuid.New().String(),
		Server: server,
	}
	sess.ctx, sess.ctxCancel = context.WithCancel(cxt)
	return sess
}

func (s *Session) Write(res *Response) (err error) {
	if res != nil {
		if err = s.Server.Transport().Response(s.Conn, res); err != nil {
			s.ctxCancel()
		}
	}
	return err
}

func (s *Session) Close() {
	s.ctxCancel()
}

func (s *Session) Context() context.Context {
	return s.ctx
}
