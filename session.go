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

func (s *Session) Write(output Output) (int, error) {
	if output != nil {
		return s.packer.Write(s.Client, output)
	}
	return 0, nil
}