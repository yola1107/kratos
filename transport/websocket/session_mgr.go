package websocket

import (
	"context"
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
		log.Infof("key=%s deleted. sessions=%d", session.ID(), count)
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

func (s *SessionManager) CloseAllSessions(ctx context.Context) error {
	// 1. 关闭所有会话
	var wg sync.WaitGroup
	s.sessions.Range(func(k, v interface{}) bool {
		session := v.(*Session)
		wg.Add(1)
		go func() {
			defer wg.Done()
			session.Close(true)
		}()
		return true
	})

	// 2. 等待会话关闭完成
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
