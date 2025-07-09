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
	"github.com/yola1107/kratos/v2/library/ext"
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
	defer ext.RecoverFromError(nil)
	defer s.Close(false)

	for {
		if s.Closed() {
			return
		}
		_ = s.conn.SetReadDeadline(time.Now().Add(s.config.ReadDeadline))
		msgType, data, err := s.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) &&
				!isNetworkClosedError(err) {
				log.Warnf("sessionID=%q read error: %v", s.id, err)
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
					log.Warnf("sessionID=%q write error: %v", s.id, err)
				}
				s.Close(true)
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
				s.Close(true)
				return
			}
			if err := s.writeMessage(websocket.BinaryMessage, pingData); err != nil {
				if !isNetworkClosedError(err) {
					log.Errorf("sessionID=%q heartbeat write error: %v", s.id, err)
				}
				s.Close(true)
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
	s.connMu.Lock()
	defer s.connMu.Unlock()
	return s.conn.WriteControl(msgType, data, time.Now().Add(s.config.WriteTimeout))
}

func (s *Session) Close(force bool) bool {
	closed := false
	s.closeOnce.Do(func() {
		closed = true
		s.closed.Store(true)

		s.cancel()

		_ = s.writeControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, closeReason(s, force)))

		// 确保不再写入
		s.connMu.Lock()
		_ = s.conn.Close()
		s.connMu.Unlock()

		close(s.sendChan)
		s.h.OnSessionClose(s)
	})
	return closed
}

func closeReason(s *Session, force bool) string {
	if force {
		if time.Since(s.LastActive()) > s.config.ReadDeadline {
			return "Force Close: Heartbeat Timeout"
		}
		return "Force Close"
	}
	return "Normal Close"
}

func isNetworkClosedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return errors.Is(err, errSessionClosed) ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "use of closed network") ||
		strings.Contains(msg, "connection closed") ||
		strings.Contains(msg, "close sent") ||
		strings.Contains(msg, "EOF")
}
