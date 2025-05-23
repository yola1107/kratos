package websocket

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	gproto "github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/matoous/go-nanoid/v2"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket/proto"
	"golang.org/x/time/rate"
)

var (
	errSessionClosed     = errors.New("session already closed")
	errWriteTimeout      = errors.New("write timeout")
	errRateLimitExceeded = errors.New("rate limit exceeded")
)

type iHandler interface {
	onClose(*Session)
	dispatch(sess *Session, data []byte) error
}

type SessionConfig struct {
	Timeout      time.Duration
	WriteTimeout time.Duration
	Interval     time.Duration
	Deadline     time.Duration
	Threshold    time.Duration
	RateLimit    int
	BurstLimit   int
	SendChanSize int
}

// Session 表示一个WebSocket连接会话
type Session struct {
	id          string
	h           iHandler
	connMu      sync.Mutex
	conn        *websocket.Conn
	config      *SessionConfig
	sendChan    chan []byte
	closeChan   chan struct{}
	closed      atomic.Bool
	lastActive  atomic.Value // time.Time
	rateLimiter *rate.Limiter
}

//生成 10 字符长度的 唯一ID
func newNanoID() string {
	shortID, _ := gonanoid.New(10)
	return shortID
}

// NewSession 创建新的WebSocket会话
func NewSession(h iHandler, conn *websocket.Conn, config *SessionConfig) *Session {
	s := &Session{
		id:          newNanoID(),
		h:           h,
		config:      config,
		conn:        conn,
		sendChan:    make(chan []byte, config.SendChanSize),
		closeChan:   make(chan struct{}),
		rateLimiter: rate.NewLimiter(rate.Limit(config.RateLimit), config.BurstLimit),
	}
	s.lastActive.Store(time.Now())
	go s.writePump()
	go s.readPump()
	go s.sendHeartbeat()
	return s
}

// ID 返回会话唯一标识
func (s *Session) ID() string {
	return s.id
}

func (s *Session) GetRemoteIP() string {
	return s.conn.RemoteAddr().String()
}

// setLastActive 设置最后活跃时间
func (s *Session) setLastActive() {
	s.lastActive.Store(time.Now())
}

// LastActive 返回最后活跃时间
func (s *Session) LastActive() time.Time {
	return s.lastActive.Load().(time.Time)
}

// Closed 检查会话是否已关闭
func (s *Session) Closed() bool {
	return s == nil || s.closed.Load()
}

// Send 发送消息到客户端
func (s *Session) Send(message []byte) error {
	if s.Closed() {
		return errSessionClosed
	}

	if !s.rateLimiter.Allow() {
		return errRateLimitExceeded
	}

	select {
	case s.sendChan <- message:
		return nil
	case <-s.closeChan:
		log.Infof("session.ID=%+v send closes ", s.id)
		return errSessionClosed
	case <-time.After(s.config.WriteTimeout):
		return errWriteTimeout
	}
}

func (s *Session) writePump() {
	for {
		select {
		case data, ok := <-s.sendChan:
			if !ok {
				return
			}
			if err := s.writeMessageLocked(data); err != nil {
				log.Errorf("session.ID=%+v write error: %v", s.id, err)
				return
			}
		case <-s.closeChan:
			return
		}
	}
}

func (s *Session) readPump() {
	defer s.Close(false)

	for {
		s.connMu.Lock()
		err := s.conn.SetReadDeadline(time.Now().Add(s.config.Deadline))
		s.connMu.Unlock()
		if err != nil {
			log.Errorf("session.ID=%+v set read deadline error: %v", s.id, err)
			return
		}

		messageType, data, err := s.conn.ReadMessage()
		if err != nil {
			if s.Closed() {
				return
			}
			if !websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warnf("session.ID=%s unexpected close: %v", s.id, err)
			}
			return
		}

		s.setLastActive()

		switch messageType {
		case websocket.BinaryMessage:
			if err := s.h.dispatch(s, data); err != nil {
				log.Errorf("session.ID=%s dispatch error: %v", s.id, err)
			}

		case websocket.PingMessage:
			s.connMu.Lock()
			err = s.conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(s.config.WriteTimeout))
			s.connMu.Unlock()
			if err != nil {
				return
			}

		case websocket.PongMessage:

		case websocket.CloseMessage:
			return

		default:
			log.Warnf("session.ID=%s unsupported message type: %d", s.id, messageType)
		}
	}
}

func (s *Session) sendHeartbeat() {
	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.connMu.Lock()
			err := s.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(s.config.WriteTimeout))
			s.connMu.Unlock()
			if err != nil {
				s.Close(true)
				return
			}
		case <-s.closeChan:
			return
		}
	}
}

func (s *Session) keepAlive() {
	if s.Closed() {
		return
	}
	// 超时检测
	if time.Since(s.LastActive()) > s.config.Deadline {
		log.Warnf("session.ID=%s heartbeat dead line.", s.id)
		s.Close(true)
		return
	}

	// 主动心跳检测
	if time.Since(s.LastActive()) > s.config.Threshold {
		log.Warnf("session.ID=%s heartbeat threshold. send ping", s.id)
		if err := s.Send(mustMarshal(&proto.Payload{Type: int32(proto.Ping)})); err != nil {
			log.Errorf("Send ping failed: %v", err)
		}
		return
	}
}

// Push 发送protobuf格式的推送消息
func (s *Session) Push(ops int32, msg gproto.Message) error {
	body := &proto.Body{
		Ops:  ops,
		Data: mustMarshal(msg),
	}
	payload := &proto.Payload{
		Type: int32(proto.Push),
		Body: mustMarshal(body),
	}
	return s.Send(mustMarshal(payload))
}

// Close 优雅关闭会话
func (s *Session) Close(force bool) bool {
	if !s.closed.CompareAndSwap(false, true) {
		return false
	}

	s.connMu.Lock()
	err := s.conn.Close()
	s.connMu.Unlock()
	if err != nil {
		log.Errorf("session.ID=%s close error: %v", s.id, err)
	}

	log.Infof("session.ID=%+v closed. force(%+v)", s.id, force)

	s.h.onClose(s)

	close(s.closeChan)
	return true
}

func (s *Session) writeMessageLocked(data []byte) error {
	if s.Closed() {
		return errSessionClosed
	}
	s.connMu.Lock()
	defer s.connMu.Unlock()

	if err := s.conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout)); err != nil {
		return err
	}
	return s.conn.WriteMessage(websocket.BinaryMessage, data)
}

func mustMarshal(pb gproto.Message) []byte {
	data, err := gproto.Marshal(pb)
	if err != nil {
		log.Errorf("marshal error: %+v", err)
		return nil
	}
	return data
}
