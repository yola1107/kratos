package websocket

import (
	"sync"
	"sync/atomic"

	"github.com/yola1107/kratos/v2/log"
)

const DefaultBucketSize = 32

// SessionManager 会话管理器，使用分桶减少锁竞争
type SessionManager struct {
	buckets    []*SessionBucket
	bucketSize int
	count      atomic.Int64
}

// SessionBucket 会话桶
type SessionBucket struct {
	sync.RWMutex
	sessions map[string]*Session
}

// NewSessionManager 创建会话管理器
func NewSessionManager() *SessionManager {
	return NewSessionManagerWithBucket(DefaultBucketSize)
}

// NewSessionManagerWithBucket 创建指定桶数量的会话管理器
func NewSessionManagerWithBucket(bucketSize int) *SessionManager {
	m := &SessionManager{
		buckets:    make([]*SessionBucket, bucketSize),
		bucketSize: bucketSize,
	}
	for i := 0; i < bucketSize; i++ {
		m.buckets[i] = &SessionBucket{
			sessions: make(map[string]*Session),
		}
	}
	return m
}

func (m *SessionManager) bucket(key string) *SessionBucket {
	return m.buckets[fnv32(key)%uint32(m.bucketSize)]
}

// fnv32 FNV-1a 哈希
func fnv32(key string) uint32 {
	h := uint32(2166136261)
	const prime32 = uint32(16777619)
	for i := 0; i < len(key); i++ {
		h = (h ^ uint32(key[i])) * prime32
	}
	return h
}

// Len 返回会话总数
func (m *SessionManager) Len() int32 {
	return int32(m.count.Load())
}

// Add 添加会话
func (m *SessionManager) Add(session *Session) {
	b := m.bucket(session.ID())
	b.Lock()
	if _, exists := b.sessions[session.ID()]; !exists {
		b.sessions[session.ID()] = session
		m.count.Add(1)
	}
	b.Unlock()

	if session.conn != nil {
		log.Infof("[websocket] session connected: id=%s, remote=%s, total=%d",
			session.ID(), session.GetRemoteIP(), m.Len())
	} else {
		log.Infof("[websocket] session connected: id=%s, total=%d",
			session.ID(), m.Len())
	}
}

// Delete 删除会话
func (m *SessionManager) Delete(session *Session) {
	b := m.bucket(session.ID())
	b.Lock()
	if _, exists := b.sessions[session.ID()]; exists {
		delete(b.sessions, session.ID())
		m.count.Add(-1)
	}
	b.Unlock()

	log.Infof("[websocket] session disconnected: id=%s, total=%d",
		session.ID(), m.Len())
}

// Get 获取会话
func (m *SessionManager) Get(sessionID string) *Session {
	b := m.bucket(sessionID)
	b.RLock()
	session := b.sessions[sessionID]
	b.RUnlock()
	return session
}

// ForEach 遍历所有会话
func (m *SessionManager) ForEach(fn func(*Session)) {
	for _, b := range m.buckets {
		b.RLock()
		for _, session := range b.sessions {
			fn(session)
		}
		b.RUnlock()
	}
}

// Broadcast 并行广播消息
func (m *SessionManager) Broadcast(data []byte) {
	var wg sync.WaitGroup
	wg.Add(m.bucketSize)

	for i := 0; i < m.bucketSize; i++ {
		go func(b *SessionBucket) {
			defer wg.Done()
			b.RLock()
			for _, session := range b.sessions {
				if !session.Closed() {
					session.Send(data)
				}
			}
			b.RUnlock()
		}(m.buckets[i])
	}

	wg.Wait()
}

// CloseAllSessions 关闭所有会话
func (m *SessionManager) CloseAllSessions() {
	var wg sync.WaitGroup
	wg.Add(m.bucketSize)

	for i := 0; i < m.bucketSize; i++ {
		go func(b *SessionBucket) {
			defer wg.Done()
			b.Lock()
			for _, session := range b.sessions {
				session.Close(true, "server shutdown")
			}
			b.Unlock()
		}(m.buckets[i])
	}

	wg.Wait()
}
