package websocket

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/yola1107/kratos/v2/library/xgo"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket/proto"
	gproto "google.golang.org/protobuf/proto"
)

var (
	errSessionClosed = errors.New("session: closed")
	errNilPayload    = errors.New("session: nil payload")
)

type iHandler interface {
	OnSessionOpen(sess *Session)
	OnSessionClose(sess *Session)
	DispatchMessage(sess *Session, data []byte) error
}

type SessionConfig struct {
	WriteTimeout time.Duration
	PingInterval time.Duration
	ReadDeadline time.Duration
	SendChanSize int
}

type Session struct {
	id        string
	conn      *websocket.Conn
	h         iHandler
	config    *SessionConfig
	ctx       context.Context
	cancel    context.CancelFunc
	sendChan  chan []byte
	lastAct   atomic.Value
	closed    atomic.Bool
	closeOnce sync.Once
	connMu    sync.Mutex
}

func NewSession(h iHandler, conn *websocket.Conn, cfg *SessionConfig) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{
		id:       uuid.NewString(),
		conn:     conn,
		h:        h,
		config:   cfg,
		ctx:      ctx,
		cancel:   cancel,
		sendChan: make(chan []byte, cfg.SendChanSize),
	}
	s.lastAct.Store(time.Now())
	h.OnSessionOpen(s)
	go s.readLoop()
	go s.writeLoop()
	go s.heartbeat()
	return s
}

func (s *Session) ID() string            { return s.id }
func (s *Session) Closed() bool          { return s.closed.Load() }
func (s *Session) LastActive() time.Time { return s.lastAct.Load().(time.Time) }
func (s *Session) GetRemoteIP() string   { return s.conn.RemoteAddr().String() }

func (s *Session) Send(data []byte) error {
	if s.Closed() {
		return errSessionClosed
	}

	select {
	case s.sendChan <- data:
		return nil
	case <-s.ctx.Done():
		return errSessionClosed
	default:
		// 防止卡死或丢包时阻塞
		log.Warnf("sessionID=%q sendChan full, dropping message", s.id)
		return errSessionClosed
	}
}

func (s *Session) SendPayload(payload *proto.Payload) error {
	if payload == nil {
		return errNilPayload
	}
	data, err := gproto.Marshal(payload)
	if err != nil {
		return err
	}
	return s.Send(data)
}

func (s *Session) Push(cmd int32, msg gproto.Message) error {
	if msg == nil {
		return errNilPayload
	}
	body, err := gproto.Marshal(msg)
	if err != nil {
		return err
	}
	return s.SendPayload(&proto.Payload{
		Op:      proto.OpPush,
		Place:   proto.PlaceServer,
		Command: cmd,
		Body:    body,
	})
}

func (s *Session) readLoop() {
	defer xgo.RecoverFromError(nil)
	defer s.Close(false)

	for {
		if s.Closed() {
			return
		}
		_ = s.conn.SetReadDeadline(time.Now().Add(s.config.ReadDeadline))
		msgType, data, err := s.conn.ReadMessage()
		if err != nil {
			if !isNetworkClosedError(err) {
				log.Warnf("sessionID=%q, %v", s.id, err)
			}
			return
		}
		s.lastAct.Store(time.Now())

		switch msgType {
		case websocket.BinaryMessage:
			if err := s.h.DispatchMessage(s, data); err != nil {
				log.Warnf("sessionID=%q dispatch error: %v", s.id, err)
			}
		case websocket.PingMessage:
			_ = s.writeControl(websocket.PongMessage, data)
		case websocket.PongMessage:
		case websocket.CloseMessage:
			return
		default:
			log.Warnf("sessionID=%q unsupported message type: %d", s.id, msgType)
		}
	}
}

func (s *Session) writeLoop() {
	defer s.Close(false)

	for {
		select {
		case <-s.ctx.Done():
			return
		case msg, ok := <-s.sendChan:
			if !ok {
				return
			}
			if err := s.writeMessage(websocket.BinaryMessage, msg); err != nil {
				if !isNetworkClosedError(err) {
					log.Warnf("sessionID=%q, %v", s.id, err)
				}
				return
			}
		}
	}
}

func (s *Session) heartbeat() {
	ticker := time.NewTicker(s.config.PingInterval)
	defer ticker.Stop()

	pingData, _ := gproto.Marshal(&proto.Payload{Op: proto.OpPing})

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if s.Closed() {
				return
			}
			if time.Since(s.LastActive()) > s.config.ReadDeadline {
				log.Warnf("sessionID=%q heartbeat timeout", s.id)
				s.Close(true, "Heartbeat Timeout")
				return
			}
			if err := s.writeMessage(websocket.BinaryMessage, pingData); err != nil {
				if !isNetworkClosedError(err) {
					log.Errorf("sessionID=%q heartbeat write error: %v", s.id, err)
				}
				s.Close(false)
				return
			}
		}
	}
}

func (s *Session) writeMessage(msgType int, data []byte) error {
	if s.Closed() {
		return errSessionClosed
	}
	s.connMu.Lock()
	defer s.connMu.Unlock()

	if err := s.conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout)); err != nil {
		return err
	}
	return s.conn.WriteMessage(msgType, data)
}

func (s *Session) writeControl(msgType int, data []byte) error {
	if s.Closed() {
		return errSessionClosed
	}
	s.connMu.Lock()
	defer s.connMu.Unlock()
	return s.conn.WriteControl(msgType, data, time.Now().Add(s.config.WriteTimeout))
}

func (s *Session) Close(force bool, msg ...string) bool {
	closed := false
	s.closeOnce.Do(func() {
		closed = true
		s.closed.Store(true)
		s.cancel()

		// 避免sendChan竞争 由ctx关闭send调用
		// close(s.sendChan)

		reason := websocket.FormatCloseMessage(websocket.CloseNormalClosure, closeReason(s, force, msg...))

		s.connMu.Lock()
		_ = s.conn.WriteControl(websocket.CloseMessage, reason, time.Now().Add(s.config.WriteTimeout))
		_ = s.conn.Close()
		s.connMu.Unlock()

		s.h.OnSessionClose(s)
	})
	return closed
}

func closeReason(s *Session, force bool, msg ...string) string {
	reason := "Normal Close"
	if force {
		reason = "Force Close"
	}
	if len(msg) > 0 {
		return reason + ": " + strings.Join(msg, "; ")
	}
	return reason
}

// 判断错误是否为连接已关闭或断开的错误。
func isNetworkClosedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()

	return errors.Is(err, errSessionClosed) || // 自定义的 session 已关闭错误
		// gorilla/websocket 标准的关闭错误码，表示连接关闭流程中的正常状态
		websocket.IsCloseError(err,
			websocket.CloseGoingAway,       // 对端关闭连接，例如浏览器关闭页面
			websocket.CloseNormalClosure,   // 正常关闭
			websocket.CloseAbnormalClosure, // 异常关闭，但属于关闭流程
		) ||
		// 低层网络错误，通常为写操作时对端关闭连接导致
		strings.Contains(msg, "broken pipe") || // 断开的管道，写时连接断开
		strings.Contains(msg, "connection reset") || // 连接被重置，通常对端关闭
		strings.Contains(msg, "use of closed network") || // 使用已关闭的连接
		strings.Contains(msg, "connection closed") || // 连接关闭
		strings.Contains(msg, "close sent") || // 已发送关闭帧
		strings.Contains(msg, "EOF") // 读到文件末尾，连接关闭
}
