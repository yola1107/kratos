package websocket

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	gproto "github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket/proto"
	"golang.org/x/time/rate"
)

var (
	errSessionClosed     = errors.New("session is closed")
	errWriteTimeout      = errors.New("write timeout")
	errRateLimitExceeded = errors.New("rate limit exceeded")
)

// Session 表示一个WebSocket连接会话
type Session struct {
	id       string
	server   *Server
	conn     *websocket.Conn
	connMu   sync.RWMutex
	values   sync.Map
	sendChan chan []byte

	ctx       context.Context
	cancel    context.CancelFunc
	closeCh   chan struct{}
	closed    atomic.Bool
	closeOnce sync.Once
	wg        sync.WaitGroup

	lastActive  atomic.Value // time.Time
	rateLimiter *rate.Limiter
}

// NewSession 创建新的WebSocket会话
func NewSession(server *Server, conn *websocket.Conn) *Session {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Session{
		id:          uuid.New().String(),
		server:      server,
		conn:        conn,
		values:      sync.Map{},
		sendChan:    make(chan []byte, server.opts.sendChannelSize),
		ctx:         ctx,
		cancel:      cancel,
		closeCh:     make(chan struct{}),
		rateLimiter: rate.NewLimiter(rate.Limit(server.opts.rateLimit), server.opts.burstLimit),
	}
	s.lastActive.Store(time.Now())

	// 设置连接参数
	conn.SetReadLimit(server.opts.maxMessageSize)
	conn.SetPongHandler(func(string) error {
		s.updateHeartbeat()
		return nil
	})

	return s
}

// ID 返回会话唯一标识
func (s *Session) ID() string {
	return s.id
}

// Set 设置会话键值对
func (s *Session) Set(key string, value interface{}) {
	s.values.Store(key, value)
}

// Get 获取会话值
func (s *Session) Get(key string) (interface{}, bool) {
	return s.values.Load(key)
}

// Metadata 获取所有元数据
func (s *Session) Metadata() map[string]interface{} {
	metadata := make(map[string]interface{})
	s.values.Range(func(k, v interface{}) bool {
		metadata[k.(string)] = v
		return true
	})
	return metadata
}

// listen 启动会话监听
func (s *Session) listen() {
	s.wg.Add(2)
	go s.writePump()
	go s.readPump()
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
	case <-s.closeCh:
		return errSessionClosed
	case <-time.After(s.server.opts.writeTimeout):
		return errWriteTimeout
	}
}

func (s *Session) writePump() {
	defer s.wg.Done()
	defer s.Close()

	for {
		select {
		case <-s.closeCh:
			return

		case data, ok := <-s.sendChan:
			if !ok {
				return
			}

			s.connMu.RLock()
			err := s.writeMessageLocked(data)
			s.connMu.RUnlock()

			if err != nil {
				log.Errorf("write error: %v", err)
				return
			}
		}
	}
}

func (s *Session) readPump() {
	defer s.wg.Done()
	defer s.Close()

	for {
		s.connMu.RLock()
		err := s.conn.SetReadDeadline(time.Now().Add(s.server.opts.heartDeadline))
		s.connMu.RUnlock()
		if err != nil {
			log.Errorf("set read deadline error: %v", err)
			return
		}

		messageType, data, err := s.conn.ReadMessage()
		if err != nil {
			if !websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Warnf("unexpected close: %v", err)
			}
			return
		}

		//log.Infof("recv. data=%+v type=%+v ", data, messageType)

		s.updateHeartbeat()

		switch messageType {
		case websocket.BinaryMessage:
			if err := s.server.dispatchMessage(s, data); err != nil {
				log.Errorf("dispatch error: %v", err)
			}

		case websocket.PingMessage:
			s.connMu.RLock()
			_ = s.conn.WriteMessage(websocket.PongMessage, nil)
			s.connMu.RUnlock()

		case websocket.CloseMessage:
			return

		default:
			log.Warnf("unsupported message type: %d", messageType)
		}
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
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		s.closed.Store(true)

		// 1. 关闭chan
		close(s.sendChan)
		close(s.closeCh)

		// 2. 取消上下文
		s.cancel()

		// 3. 发送关闭帧
		s.sendCloseFrame()

		// 4. 等待协程退出
		s.waitForGoroutines()

		// 5. 关闭底层连接
		s.closeUnderlyingConn()

		// 6. 从服务器注销
		s.unregisterFromServer()
	})
}

// Closed 检查会话是否已关闭
func (s *Session) Closed() bool {
	return s.closed.Load()
}

// LastActive 返回最后活跃时间
func (s *Session) LastActive() time.Time {
	return s.lastActive.Load().(time.Time)
}

func (s *Session) sendCloseFrame() {
	done := make(chan struct{})
	go func() {
		s.connMu.Lock()
		defer s.connMu.Unlock()
		defer close(done)

		if s.conn != nil {
			_ = s.conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(s.server.opts.writeTimeout),
			)
		}
	}()
	select {
	case <-done:
	case <-time.After(s.server.opts.closeGracePeriod):
		log.Warnf("key %+v close frame timeout", s.id)
	}
}

func (s *Session) waitForGoroutines() {
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(s.server.opts.shutdownTimeout):
		log.Warnf("key: %s close timeout", s.id)
	}
}

func (s *Session) closeUnderlyingConn() {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}

func (s *Session) unregisterFromServer() {
	select {
	case s.server.unregister <- s:
	case <-time.After(s.server.opts.shutdownTimeout):
		log.Warnf("key: %s unregister timeout", s.id)
	}
}

func (s *Session) writeMessageLocked(data []byte) error {
	if s.conn == nil {
		return errSessionClosed
	}

	if err := s.conn.SetWriteDeadline(time.Now().Add(s.server.opts.writeTimeout)); err != nil {
		return err
	}

	return s.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (s *Session) updateHeartbeat() {
	s.lastActive.Store(time.Now())
}

func mustMarshal(pb gproto.Message) []byte {
	data, err := gproto.Marshal(pb)
	if err != nil {
		log.Errorf("marshal error: %+v", err)
		return nil
	}
	return data
}
