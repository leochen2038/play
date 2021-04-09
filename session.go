package play

import (
	"github.com/google/uuid"
)

type Session struct {
	SessId string
	UserInfo interface{}
	Conn *Conn
	packer Packer
}

func NewSession(c *Conn, packer Packer) *Session {
	return &Session{
		Conn: c,
		SessId: uuid.New().String(),
		packer: packer,
	}
}

func (s *Session) Write(output Output)  error {
	if output != nil {
		return s.packer.Write(s.Conn, output)
	}
	return nil
}

func (s *Session)Close() {
	s.Conn.IsClose = true
}