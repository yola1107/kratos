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

func (m *SessionManager) Len() int32 {
	return atomic.LoadInt32(&m.count)
}

func (m *SessionManager) Add(session *Session) {
	if _, loaded := m.sessions.LoadOrStore(session.ID(), session); !loaded {
		atomic.AddInt32(&m.count, 1)
		if session.conn != nil {
			log.Infof("start ws serve %q with %q key=%q sessions=%d",
				session.conn.LocalAddr(), session.conn.RemoteAddr(), session.ID(), atomic.LoadInt32(&m.count))
		} else {
			log.Infof("start ws serve key=%q sessions=%d", session.ID(), atomic.LoadInt32(&m.count))
		}
	}
}

func (m *SessionManager) Delete(session *Session) {
	if _, loaded := m.sessions.LoadAndDelete(session.ID()); loaded {
		atomic.AddInt32(&m.count, -1)
		log.Infof("disconnected key=%q sessions=%d", session.ID(), atomic.LoadInt32(&m.count))
	}
}

func (m *SessionManager) Get(sessionId string) *Session {
	v, ok := m.sessions.Load(sessionId)
	if !ok {
		return nil
	}
	session, ok := v.(*Session)
	if !ok {
		log.Errorf("Invalid session type: key=%s", sessionId)
		m.sessions.Delete(sessionId) // 自动清理无效数据
		return nil
	}
	return session
}

func (m *SessionManager) ForEach(fn func(*Session)) {
	m.sessions.Range(func(k, v interface{}) bool {
		if session, ok := v.(*Session); ok {
			fn(session)
		}
		return true
	})
}

func (m *SessionManager) Broadcast(data []byte) {
	m.sessions.Range(func(k, v interface{}) bool {
		if session, ok := v.(*Session); ok {
			if !session.Closed() {
				if err := session.Send(data); err != nil {
					log.Errorf("Broadcast failed: %v", err)
				}
			}
		}
		return true
	})
}

func (m *SessionManager) BroadcastAsync(data []byte) {
	m.sessions.Range(func(_, v interface{}) bool {
		session := v.(*Session)
		// 异步发送，避免阻塞Range操作
		go func(s *Session) {
			if !s.Closed() {
				if err := s.Send(data); err != nil {
					log.Errorf("BroadcastAsync failed for session %s: %v", s.ID(), err)
				}
			}
		}(session)
		return true
	})
}

func (m *SessionManager) CloseAllSessions() {
	m.sessions.Range(func(_, v interface{}) bool {
		if session, ok := v.(*Session); ok {
			session.Close(true, "server closed")
		}
		return true
	})
	return
}
