package websocket

import (
	"sync"
	"sync/atomic"

	"github.com/yola1107/kratos/v2/log"
)

type SessionManager struct {
	count    int32
	sessions sync.Map // sessionID -> *Session
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
		log.Infof("key=%s deleted. sessions=%d", session.ID(), count)
	}
}

func (s *SessionManager) Get(sessionId string) *Session {
	v, ok := s.sessions.Load(sessionId)
	if !ok {
		return nil
	}
	if session, ok := v.(*Session); ok {
		return session
	}
	log.Warnf("未知类型存储 sessionID:%+v", sessionId)
	return nil
}

func (s *SessionManager) Range(fn func(*Session)) {
	s.sessions.Range(func(k, v interface{}) bool {
		if session, ok := v.(*Session); ok {
			fn(session)
		}
		return true
	})
}
func (s *SessionManager) CloseAllSessions() {
	s.sessions.Range(func(_, v interface{}) bool {
		if session, ok := v.(*Session); ok {
			session.Close(true)
		}
		return true
	})
	return
}
