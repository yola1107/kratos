package websocket

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yola1107/kratos/v2/library/xgo"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket/proto"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	gproto "google.golang.org/protobuf/proto"
)

const (
	normalCloseReason = "Normal Close"
	forceCloseReason  = "Force Close"
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
	return s.SendPayload(&proto.Payload{Op: proto.OpPush, Place: proto.PlaceServer, Command: cmd, Body: body})
}

func (s *Session) readLoop() {
	defer xgo.RecoverFromError(nil)
	defer s.Close(false)

	for !s.Closed() {
		s.conn.SetReadDeadline(time.Now().Add(s.config.ReadDeadline))
		msgType, data, err := s.conn.ReadMessage()
		if err != nil {
			if !isNetworkClosedError(err) {
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
			s.writeControl(websocket.PongMessage, data)
		case websocket.CloseMessage:
			return
		}
	}
}

func (s *Session) writeLoop() {
	defer func() {
		// 只有在非正常关闭时才调用 Close
		if !s.Closed() {
			s.Close(false, "writeLoop exit")
		}
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		case msg, ok := <-s.sendChan:
			if !ok {
				// sendChan 被关闭，退出循环
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
			if s.Closed() || time.Since(s.LastActive()) > s.config.ReadDeadline {
				if !s.Closed() {
					log.Warnf("sessionID=%q heartbeat timeout", s.id)
					s.Close(true, "Heartbeat Timeout")
				}
				return
			}
			if err := s.writeMessage(websocket.BinaryMessage, pingData); err != nil && !isNetworkClosedError(err) {
				log.Errorf("sessionID=%q heartbeat error: %v", s.id, err)
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
	return s.writeWithDeadline(func() error { return s.conn.WriteMessage(msgType, data) })
}

func (s *Session) writeControl(msgType int, data []byte) error {
	if s.Closed() {
		return errSessionClosed
	}
	s.connMu.Lock()
	defer s.connMu.Unlock()
	return s.conn.WriteControl(msgType, data, time.Now().Add(s.config.WriteTimeout))
}

func (s *Session) writeWithDeadline(fn func() error) error {
	deadline := time.Now().Add(s.config.WriteTimeout)
	if err := s.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	return fn()
}

func (s *Session) Close(force bool, msg ...string) bool {
	closed := false
	s.closeOnce.Do(func() {
		closed = true
		s.closed.Store(true)
		s.cancel()

		// Close send channel safely
		defer func() { recover() }() // ignore panic if already closed
		close(s.sendChan)

		// Send close frame and close connection
		if s.conn != nil {
			reason := websocket.FormatCloseMessage(websocket.CloseNormalClosure, closeReason(s, force, msg...))
			s.connMu.Lock()
			s.conn.WriteControl(websocket.CloseMessage, reason, time.Now().Add(s.config.WriteTimeout))
			s.conn.Close()
			s.connMu.Unlock()
		}

		// Notify handler
		if s.h != nil {
			s.h.OnSessionClose(s)
		}

		log.Infof("session closed: id=%s, reason=%s", s.id, closeReason(s, force, msg...))
	})
	return closed
}

func closeReason(s *Session, force bool, msg ...string) string {
	reason := normalCloseReason
	if force {
		reason = forceCloseReason
	}
	if len(msg) > 0 {
		reason += ": " + strings.Join(msg, "; ")
	}
	return reason
}

// isNetworkClosedError checks if error indicates network connection is closed
func isNetworkClosedError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, errSessionClosed) ||
		websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
		return true
	}

	msg := err.Error()
	closedMsgs := []string{"broken pipe", "connection reset", "use of closed network", "connection closed", "close sent", "EOF"}
	for _, closedMsg := range closedMsgs {
		if strings.Contains(msg, closedMsg) {
			return true
		}
	}
	return false
}
