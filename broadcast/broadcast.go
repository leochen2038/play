package broadcast

import (
	"errors"
	"github.com/leochen2038/play"
	"sync"
)

type messageChan struct {
	message play.Output
	excluded *play.Session
	feedback chan<- Feedback
}

type Feedback struct {
	Session *play.Session
	Err error
}

type group struct {
	name string
	members sync.Map
	messageChannel chan messageChan
}

var groupList sync.Map

func (g *group)broadcast() {
	go func() {
		defer func() {
			// 异常处理
		}()
		for {
			select {
			case message := <- g.messageChannel:
				g.members.Range(func(key, value interface{}) bool {
					s := value.(*play.Session)
					if message.excluded != nil && message.excluded.SessId != s.SessId {
						err := s.Write(message.message)
						if err != nil {
							g.members.Delete(key)
						}
						if message.feedback != nil {
							message.feedback <- Feedback{Session: s, Err: err}
						}
					}
					return true
				})
				if message.feedback != nil {
					close(message.feedback)
				}
			}
		}
	}()
}

func AddMember(name string, session *play.Session) {
	v, ok := groupList.Load(name)
	if !ok {
		vv := &group{name: name}
		vv.broadcast()
		groupList.Store(name, vv)
	}
	v.(*group).members.Store(session.SessId, session)
}

func Broadcast(name string, output play.Output, excluded *play.Session, feedback bool) (<-chan Feedback, error) {
	var err error
	var feedbackChan chan Feedback = nil
	if v, ok := groupList.Load(name); ok {
		if feedback == true {
			feedbackChan = make(chan Feedback, 32)
		}
		v.(*group).messageChannel <- messageChan{message: output, excluded: excluded, feedback: feedbackChan}
	} else {
		err = errors.New("unable find group:"+name)
	}
	return feedbackChan, err
}