package play

import (
	"github.com/google/uuid"
)

type Session struct {
	SessId string
	UserInfo interface{}
	Client *Client
	packer Packer
}

func NewSession(c *Client, packer Packer) *Session {
	return &Session{
		Client: c,
		SessId: uuid.New().String(),
		packer: packer,
	}
}

func (s *Session) Write(output Output)  error {
	if output != nil {
		return s.packer.Write(s.Client, output)
	}
	return nil
}

func (s *Session)Close() {
	s.Client.IsClose = true
}