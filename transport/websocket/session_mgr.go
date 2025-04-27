package websocket

import (
	"sync"
	"sync/atomic"

	"github.com/yola1107/kratos/v2/log"
)

type SessionManager struct {
	count    int32
	sessions sync.Map
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		count:    0,
		sessions: sync.Map{},
	}
}

func (s *SessionManager) Len() int32 {
	return atomic.LoadInt32(&s.count)
}

func (s *SessionManager) Add(session *Session) {
	if _, loaded := s.sessions.LoadOrStore(session.ID(), session); !loaded {
		count := atomic.AddInt32(&s.count, 1)
		log.Infof("start ws serve. \"%s\" with \"%s\" key=\"%s\" sessions=%d",
			session.conn.LocalAddr(), session.conn.RemoteAddr(), session.ID(), count)
	}
}

func (s *SessionManager) Delete(session *Session) {
	if _, loaded := s.sessions.LoadAndDelete(session.ID()); loaded {
		count := atomic.AddInt32(&s.count, -1)
		log.Infof("disconnect. key=\"%s\" sessions=%d", session.ID(), count)
	}
}

func (s *SessionManager) Get(sessionId string) *Session {
	v, ok := s.sessions.Load(sessionId)
	if !ok {
		return nil
	}
	return v.(*Session)
}

func (s *SessionManager) Range(fn func(*Session)) {
	s.sessions.Range(func(k, v interface{}) bool {
		session := v.(*Session)
		fn(session)
		return true
	})
}
