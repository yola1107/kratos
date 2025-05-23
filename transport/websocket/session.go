package websocket

import (
	"context"
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
	errSessionClosed     = errors.New("session is closed")
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
	RateLimit    int
	BurstLimit   int
	SendChanSize int
}

type Session struct {
	id          string
	h           iHandler
	connMu      sync.Mutex
	conn        *websocket.Conn
	config      *SessionConfig
	sendChan    chan []byte
	closed      atomic.Bool
	lastActive  atomic.Value // time.Time
	rateLimiter *rate.Limiter
	ctx         context.Context
	cancel      context.CancelFunc
}

func newNanoID() string {
	shortID, _ := gonanoid.New(10)
	return shortID
}

func NewSession(h iHandler, conn *websocket.Conn, config *SessionConfig) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{
		id:          newNanoID(),
		h:           h,
		conn:        conn,
		config:      config,
		sendChan:    make(chan []byte, config.SendChanSize),
		rateLimiter: rate.NewLimiter(rate.Limit(config.RateLimit), config.BurstLimit),
		ctx:         ctx,
		cancel:      cancel,
	}
	s.lastActive.Store(time.Now())
	go s.readPump()
	go s.writePump()
	go s.heartbeat()
	return s
}

func (s *Session) ID() string {
	return s.id
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
	if s.Closed() {
		return errSessionClosed
	}
	if !s.rateLimiter.Allow() {
		return errRateLimitExceeded
	}
	select {
	case s.sendChan <- message:
		return nil
	case <-time.After(s.config.WriteTimeout):
		return errWriteTimeout
	case <-s.ctx.Done():
		return errSessionClosed
	}
}

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

func (s *Session) readPump() {
	defer s.Close(false)

	for {
		if err := s.conn.SetReadDeadline(time.Now().Add(s.config.Deadline)); err != nil {
			log.Errorf("sessionID=\"%s\" set read deadline error: %v", s.id, err)
			return
		}

		msgType, data, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warnf("sessionID=\"%s\" unexpected close: %v", s.id, err)
			} else {
				log.Warnf("sessionID=\"%s\" %v", s.id, err)
			}
			return
		}

		s.lastActive.Store(time.Now())

		switch msgType {
		case websocket.BinaryMessage:
			if err := s.h.dispatch(s, data); err != nil {
				log.Errorf("sessionID=\"%s\" dispatch error: %v", s.id, err)
			}
		case websocket.PingMessage:
			s.writeControl(websocket.PongMessage, data)
		case websocket.CloseMessage:
			return
		default:
			log.Warnf("sessionID=\"%s\" unsupported message type: %d", s.id, msgType)
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
			if err := s.writeMessage(msg); err != nil {
				log.Errorf("sessionID=\"%s\" write error: %v", s.id, err)
				s.Close(true)
				return
			}
		}
	}
}

func (s *Session) heartbeat() {
	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return

		case <-ticker.C:
			if s.Closed() {
				return
			}
			if time.Since(s.LastActive()) > s.config.Deadline {
				log.Warnf("sessionID=\"%s\" heartbeat timeout", s.id)
				s.Close(true)
				return
			}
			_ = s.Send(mustMarshal(&proto.Payload{Type: int32(proto.Ping)}))
		}
	}
}

func (s *Session) Close(force bool) bool {
	if !s.closed.CompareAndSwap(false, true) {
		return false
	}

	defer RecoverFromError(nil)

	close(s.sendChan)
	s.cancel()
	s.connMu.Lock()
	err := s.conn.Close()
	s.connMu.Unlock()
	if err != nil {
		log.Errorf("sessionID=\"%s\" close conn error: %v", s.id, err)
	}

	log.Infof("sessionID=\"%s\" closed. force(%+v)", s.id, force)

	s.h.onClose(s)

	return true
}

func (s *Session) writeControl(msgType int, data []byte) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	_ = s.conn.WriteControl(msgType, data, time.Now().Add(s.config.WriteTimeout))
}

func (s *Session) writeMessage(data []byte) error {
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
