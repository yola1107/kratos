package websocket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yola1107/kratos/v2/library/xgo"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket/proto"

	"github.com/gorilla/websocket"
	gproto "google.golang.org/protobuf/proto"
)

var (
	ErrClosedRequest = errors.New("client: session not established")
	ErrMaxRetries    = errors.New("client: max retries reached")
	ErrInvalidURL    = errors.New("client: invalid URL")
)

type PushHandler func(data []byte)
type ResponseHandler func(data []byte, code int32)

// ClientOption Legacy option functions for backward compatibility
type ClientOption func(*clientOptions)

// WithEndpoint with client endpoint.
func WithEndpoint(endpoint string) ClientOption {
	return func(o *clientOptions) { o.endpoint = endpoint }
}

// WithTimeout with client timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.timeout = timeout }
}

// WithToken with authentication token.
func WithToken(token string) ClientOption {
	return func(o *clientOptions) { o.token = token }
}

// WithTLSConfig with TLS config.
func WithTLSConfig(c *tls.Config) ClientOption {
	return func(o *clientOptions) { o.tlsConf = c }
}

// WithSessionConfig with session configuration.
func WithSessionConfig(c *SessionConfig) ClientOption {
	return func(o *clientOptions) { o.session = c }
}

// WithConnectFunc with connection callback.
func WithConnectFunc(fn func(*Session)) ClientOption {
	return func(o *clientOptions) { o.connectFunc = fn }
}

// WithDisconnectFunc with disconnection callback.
func WithDisconnectFunc(fn func(*Session)) ClientOption {
	return func(o *clientOptions) { o.disconnectFunc = fn }
}

// WithPushHandler with push message handlers.
func WithPushHandler(handler map[int32]PushHandler) ClientOption {
	return func(o *clientOptions) { o.pushHandler = handler }
}

// WithResponseHandler with response message handlers.
func WithResponseHandler(handler map[int32]ResponseHandler) ClientOption {
	return func(o *clientOptions) { o.responseHandler = handler }
}

// WithRetryPolicy with reconnection policy.
func WithRetryPolicy(delay time.Duration, maxAttempt int32) ClientOption {
	return func(o *clientOptions) {
		o.retryDelay = delay
		o.retryMaxAttempt = maxAttempt
	}
}

// clientOptions is websocket client options
type clientOptions struct {
	ctx             context.Context
	tlsConf         *tls.Config
	timeout         time.Duration
	endpoint        string
	token           string
	connectFunc     func(*Session)
	disconnectFunc  func(*Session)
	pushHandler     map[int32]PushHandler
	responseHandler map[int32]ResponseHandler
	session         *SessionConfig
	retryDelay      time.Duration
	retryMaxAttempt int32
}

type Client struct {
	opts       *clientOptions
	url        *url.URL
	seq        int32
	reqPool    sync.Map // seq -> command(int32)
	session    *Session
	retryCount atomic.Int32
}

// NewClient creates a Websocket client by options.
func NewClient(ctx context.Context, opts ...ClientOption) (*Client, error) {
	// 默认配置
	options := &clientOptions{
		ctx:             ctx,
		endpoint:        "ws://0.0.0.0:3102",
		timeout:         2 * time.Second,
		pushHandler:     make(map[int32]PushHandler),
		responseHandler: make(map[int32]ResponseHandler),
		session: &SessionConfig{
			WriteTimeout: 10 * time.Second,
			PingInterval: 15 * time.Second,
			ReadDeadline: 60 * time.Second,
			SendChanSize: 32,
		},
		retryDelay:      3 * time.Second,
		retryMaxAttempt: -1, // unlimited retry
	}

	// 应用选项
	for _, o := range opts {
		o(options)
	}

	u, err := parseURL(options.endpoint, options.tlsConf == nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	c := &Client{
		opts:    options,
		url:     u,
		seq:     0,
		reqPool: sync.Map{},
	}

	// 立即尝试连接
	if err := c.Reconnect(); err != nil {
		return nil, err
	}

	return c, nil
}

func parseURL(endpoint string, insecure bool) (*url.URL, error) {
	if !strings.Contains(endpoint, "://") {
		if insecure {
			endpoint = "ws://" + endpoint
		} else {
			endpoint = "wss://" + endpoint
		}
	}
	return url.Parse(endpoint)
}

// IsAlive returns true if the client is connected
func (c *Client) IsAlive() bool {
	return c != nil && c.session != nil && !c.session.Closed()
}

func (c *Client) GetSession() *Session {
	return c.session
}

// Reconnect establishes connection with exponential backoff retry
func (c *Client) Reconnect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: c.opts.session.WriteTimeout,
		TLSClientConfig:  c.opts.tlsConf,
	}

	c.Close()

	for attempt := int32(1); ; attempt++ {
		conn, _, err := dialer.DialContext(c.opts.ctx, c.url.String(), nil)
		if err == nil {
			c.retryCount.Store(0)
			c.session = NewSession(c, conn, c.opts.session)
			return nil
		}

		if c.opts.retryMaxAttempt >= 0 && attempt >= c.opts.retryMaxAttempt {
			return fmt.Errorf("reconnect failed after %d attempts: %w", attempt, err)
		}

		delay := c.calculateBackoff(attempt)
		log.Warnf("websocket reconnect attempt %d failed, retrying in %v: %v", attempt, delay, err)

		select {
		case <-time.After(delay):
		case <-c.opts.ctx.Done():
			return fmt.Errorf("reconnect cancelled: %w", c.opts.ctx.Err())
		}
	}
}

// calculateBackoff computes exponential backoff delay
func (c *Client) calculateBackoff(attempt int32) time.Duration {
	backoff := float64(c.opts.retryDelay) * math.Pow(1.5, float64(attempt-1))
	return time.Duration(backoff * (0.9 + 0.2*rand.Float64()))
}

// OnSessionOpen 连接成功回调
func (c *Client) OnSessionOpen(sess *Session) {
	if c.opts.connectFunc != nil {
		safeCall(func() { c.opts.connectFunc(sess) })
	}
}

// OnSessionClose handles connection close and auto-reconnect
func (c *Client) OnSessionClose(sess *Session) {
	if c.opts.disconnectFunc != nil {
		safeCall(func() { c.opts.disconnectFunc(sess) })
	}

	if c.opts.retryMaxAttempt != 0 {
		go func() {
			if err := c.Reconnect(); err != nil {
				log.Warnf("reconnect failed: %v", err)
			}
		}()
	}
}

// Request sends a request message
func (c *Client) Request(command int32, msg gproto.Message) error {
	if !c.IsAlive() {
		return ErrClosedRequest
	}

	seq := atomic.AddInt32(&c.seq, 1)
	if seq >= math.MaxInt32-1 {
		atomic.StoreInt32(&c.seq, 1)
		seq = 1
	}

	data, err := gproto.Marshal(msg)
	if err != nil {
		return err
	}

	c.reqPool.Store(seq, command)
	return c.session.SendPayload(&proto.Payload{
		Op:      proto.OpRequest,
		Place:   proto.PlaceClient,
		Seq:     seq,
		Command: command,
		Body:    data,
	})
}

// DispatchMessage handles incoming messages
func (c *Client) DispatchMessage(sess *Session, data []byte) error {
	var p proto.Payload
	if err := gproto.Unmarshal(data, &p); err != nil {
		return err
	}

	switch p.Op {
	case proto.OpResponse:
		c.handleResponse(&p)
	case proto.OpPush:
		c.handlePush(&p)
	case proto.OpPing:
		return sess.SendPayload(&proto.Payload{Op: proto.OpPong})
	}

	return nil
}

// handleResponse processes response messages
func (c *Client) handleResponse(p *proto.Payload) {
	cmdInterface, loaded := c.reqPool.LoadAndDelete(p.Seq)
	if !loaded {
		return
	}

	if command, ok := cmdInterface.(int32); ok {
		if handler, exists := c.opts.responseHandler[command]; exists {
			safeCall(func() { handler(p.Body, p.Code) })
		}
	}
}

// handlePush processes push messages
func (c *Client) handlePush(p *proto.Payload) {
	if handler, exists := c.opts.pushHandler[p.Command]; exists {
		safeCall(func() { handler(p.Body) })
	}
}

// Close closes the client and cleans up resources
func (c *Client) Close(msg ...string) {
	s := c.session
	if s == nil {
		return
	}

	reason := ""
	if len(msg) > 0 {
		reason = "client closed: " + strings.Join(msg, "; ")
	}
	c.session = nil
	s.Close(true, reason)
	c.reqPool.Range(func(key, value any) bool {
		c.reqPool.Delete(key)
		return true
	})
}

// safeCall 用于安全调用回调，避免panic导致崩溃
func safeCall(fn func()) {
	defer xgo.RecoverFromError(nil)
	if fn != nil {
		fn()
	}
}
