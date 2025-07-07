package websocket

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gproto "github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/matoous/go-nanoid/v2"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket/proto"
)

var (
	errSessionClosed = errors.New("session: closed send")
	errSendNilProto  = errors.New("session: send nil payload")
)

type iHandler interface {
	// OnSessionOpen 会话建立后回调，例如注册 session、绑定用户等
	OnSessionOpen(sess *Session)
	// OnSessionClose 会话断开时回调，例如清理缓存、断开房间、注销 session 等
	OnSessionClose(sess *Session)
	// DispatchMessage 处理客户端发来的 protobuf 原始二进制数据
	DispatchMessage(sess *Session, data []byte) error
}

type SessionConfig struct {
	WriteTimeout time.Duration
	PingInterval time.Duration
	ReadDeadline time.Duration
	SendChanSize int
}

type Session struct {
	id         string
	uid        atomic.Int64
	h          iHandler
	connMu     sync.Mutex
	conn       *websocket.Conn
	config     *SessionConfig
	sendChan   chan []byte
	closed     atomic.Bool
	lastActive atomic.Value // time.Time
	ctx        context.Context
	cancel     context.CancelFunc
	sendMu     sync.Mutex
}

func newNanoID() string {
	shortID, _ := gonanoid.New(10)
	return "NANO-" + shortID
}

func NewSession(h iHandler, conn *websocket.Conn, config *SessionConfig) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{
		id:       uuid.New().String(), // newNanoID()
		h:        h,
		conn:     conn,
		config:   config,
		sendChan: make(chan []byte, config.SendChanSize),
		ctx:      ctx,
		cancel:   cancel,
	}
	s.uid.Store(int64(0))
	s.lastActive.Store(time.Now())
	s.h.OnSessionOpen(s)
	go s.readPump()
	go s.writePump()
	go s.heartbeat()
	return s
}

func (s *Session) ID() string {
	return s.id
}

func (s *Session) SetUID(uid int64) {
	s.uid.Store(uid)
}

func (s *Session) UID() int64 {
	return s.uid.Load()
}

func (s *Session) GetRemoteIP() string {
	return s.conn.RemoteAddr().String()
}

func (s *Session) LastActive() time.Time {
	return s.lastActive.Load().(time.Time)
}

func (s *Session) Closed() bool {
	return s.closed.Load()
}

func (s *Session) Send(message []byte) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	if s.Closed() {
		return errSessionClosed
	}
	select {
	case s.sendChan <- message:
		return nil
	case <-s.ctx.Done():
		return errSessionClosed
	}
}

func (s *Session) SendPayload(payload *proto.Payload) error {
	if payload == nil {
		return errSendNilProto
	}
	data, err := gproto.Marshal(payload)
	if err != nil {
		return err
	}
	return s.Send(data)
}

func (s *Session) Push(command int32, msg gproto.Message) error {
	body, err := gproto.Marshal(msg)
	if err != nil {
		return err
	}
	payload := &proto.Payload{
		Op:      proto.OpPush,
		Place:   proto.PlaceServer,
		Command: command,
		Body:    body,
	}
	return s.SendPayload(payload)
}

func (s *Session) readPump() {
	defer ext.RecoverFromError(nil)
	defer s.Close(false)

	for {
		if err := s.conn.SetReadDeadline(time.Now().Add(s.config.ReadDeadline)); err != nil {
			log.Errorf("sessionID=%q set read deadline error: %v", s.id, err)
			return
		}

		msgType, data, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warnf("sessionID=%q unexpected close: %v", s.id, err)
			} else {
				// log.Infof("sessionID=%q normal close: %v", s.id, err)
			}
			return
		}

		s.lastActive.Store(time.Now())

		switch msgType {
		case websocket.BinaryMessage:
			_ = s.h.DispatchMessage(s, data)
		case websocket.PingMessage:
			s.writeControl(websocket.PongMessage, data)
		case websocket.PongMessage:
		case websocket.CloseMessage:
			return
		default:
			log.Warnf("sessionID=%q unsupported message type: %d", s.id, msgType)
		}
	}
}

func (s *Session) writePump() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case msg, ok := <-s.sendChan:
			if !ok {
				return
			}
			if err := s.writeBinaryMessage(msg); err != nil {
				if errors.Is(err, errSessionClosed) || strings.Contains(err.Error(), "close sent") {
					log.Infof("sessionID=%q write aborted, reason: %v", s.id, err)
				} else {
					log.Errorf("sessionID=%q write error: %v", s.id, err)
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
			data, _ := gproto.Marshal(&proto.Payload{Op: proto.OpPing})
			_ = s.writeBinaryMessage(data)
		}
	}
}

func (s *Session) Close(force bool) bool {
	if !s.closed.CompareAndSwap(false, true) {
		return false
	}

	s.closeNotify(force)

	s.cancel()

	s.sendMu.Lock()
	close(s.sendChan)
	s.sendMu.Unlock()

	s.connMu.Lock()
	_ = s.conn.Close()
	s.connMu.Unlock()

	s.h.OnSessionClose(s) // 回调处理器
	return true
}

func (s *Session) closeNotify(force bool) {
	reason := "Normal Closure"
	if force {
		reason = "Force Closure"
		if time.Since(s.LastActive()) > s.config.ReadDeadline {
			reason = "Force Closure (Heartbeat timeout)"
		}
	}
	message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, reason)
	s.writeControl(websocket.CloseMessage, message)
}

func (s *Session) writeControl(msgType int, data []byte) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	_ = s.conn.WriteControl(msgType, data, time.Now().Add(s.config.WriteTimeout))
}

func (s *Session) writeBinaryMessage(data []byte) error {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	if s.Closed() {
		return errSessionClosed
	}
	if err := s.conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout)); err != nil {
		return err
	}
	return s.conn.WriteMessage(websocket.BinaryMessage, data)
}
