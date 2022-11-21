package play

import (
	"context"

	"github.com/google/uuid"
)

type Session struct {
	SessId    string
	User      interface{}
	Conn      *Conn
	Server    IServer
	ctx       context.Context
	ctxCancel context.CancelFunc
}

func NewSession(cxt context.Context, server IServer) *Session {
	sess := &Session{
		Conn:   &Conn{Type: server.Info().Type},
		SessId: uuid.New().String(),
		Server: server,
	}
	sess.ctx, sess.ctxCancel = context.WithCancel(cxt)
	return sess
}

func (s *Session) Write(res *Response) (err error) {
	if res != nil {
		var data []byte
		if data, err = s.Server.Packer().Pack(s.Conn, res); err == nil && len(data) > 0 {
			err = s.Server.Transport(s.Conn, data)
		}
	}

	if err != nil {
		s.ctxCancel()
	}
	return err
}

func (s *Session) Close() {
	s.ctxCancel()
}

func (s *Session) Context() context.Context {
	return s.ctx
}
