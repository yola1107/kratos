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

	"github.com/gorilla/websocket"
	"github.com/yola1107/kratos/v2/library/xgo"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket/proto"
	gproto "google.golang.org/protobuf/proto"
)

var (
	ErrClosedRequest = errors.New("client: session not established")
	ErrMaxRetries    = errors.New("client: max retries reached")
	ErrInvalidURL    = errors.New("client: invalid URL")
)

type PushHandler func(data []byte)
type ResponseHandler func(data []byte, code int32)

type ClientOption func(*clientOptions)

func WithTlsConf(tlsConfig *tls.Config) ClientOption {
	return func(o *clientOptions) { o.tlsConf = tlsConfig }
}

func WithTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.timeout = timeout }
}

func WithSessionConfig(c *SessionConfig) ClientOption {
	return func(o *clientOptions) { o.session = c }
}

func WithHeartbeat(readDeadline, pingInterval, writeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.session.ReadDeadline = readDeadline
		o.session.PingInterval = pingInterval
		o.session.WriteTimeout = writeTimeout
	}
}

func WithSendChanSize(size int) ClientOption {
	return func(o *clientOptions) { o.session.SendChanSize = size }
}

func WithEndpoint(endpoint string) ClientOption {
	return func(o *clientOptions) { o.endpoint = endpoint }
}

func WithToken(token string) ClientOption {
	return func(o *clientOptions) { o.token = token }
}

func WithConnectFunc(fn func(*Session)) ClientOption {
	return func(o *clientOptions) { o.connectFunc = fn }
}

func WithDisconnectFunc(fn func(*Session)) ClientOption {
	return func(o *clientOptions) { o.disconnectFunc = fn }
}

func WithPushHandler(handler map[int32]PushHandler) ClientOption {
	return func(o *clientOptions) { o.pushHandler = handler }
}

func WithResponseHandler(handler map[int32]ResponseHandler) ClientOption {
	return func(o *clientOptions) { o.responseHandler = handler }
}

func WithRetryPolicy(baseDelay, maxDelay time.Duration, maxAttempt int32) ClientOption {
	return func(o *clientOptions) {
		o.retryPolicy.baseDelay = baseDelay
		o.retryPolicy.maxDelay = maxDelay
		o.retryPolicy.maxAttempt = maxAttempt
	}
}

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
	retryPolicy     *retryPolicy
}

type retryPolicy struct {
	baseDelay  time.Duration
	maxDelay   time.Duration
	maxAttempt int32 // <0: unlimited retry
}

type Client struct {
	opts       *clientOptions
	url        *url.URL
	seq        int32
	reqPool    sync.Map // seq -> command(int32)
	session    *Session
	retryCount atomic.Int32
	wg         sync.WaitGroup
}

func NewClient(ctx context.Context, opts ...ClientOption) (*Client, error) {
	options := &clientOptions{
		ctx:             ctx,
		timeout:         2 * time.Second,
		tlsConf:         nil,
		endpoint:        "ws://0.0.0.0:3102",
		token:           "",
		connectFunc:     nil,
		disconnectFunc:  nil,
		pushHandler:     make(map[int32]PushHandler),
		responseHandler: make(map[int32]ResponseHandler),
		session: &SessionConfig{
			WriteTimeout: 10 * time.Second,
			PingInterval: 10 * time.Second,
			ReadDeadline: 60 * time.Second,
			SendChanSize: 32,
		},
		retryPolicy: &retryPolicy{
			baseDelay:  3 * time.Second,
			maxDelay:   15 * time.Second,
			maxAttempt: 0,
		},
	}

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

// IsAlive returns true if the session is established and open
func (c *Client) IsAlive() bool {
	if c == nil {
		return false
	}
	sess := c.session
	return sess != nil && !sess.Closed()
}

func (c *Client) GetSession() *Session {
	return c.session
}

// canRetry 判断是否允许重试
func (c *Client) canRetry() bool {
	maxAttempt := c.opts.retryPolicy.maxAttempt
	if maxAttempt < 0 {
		return true // 无限重试
	}
	if maxAttempt == 0 {
		return false // 不允许重试
	}

	curr := c.retryCount.Load()
	return curr < maxAttempt
}

// Reconnect 执行连接操作
func (c *Client) Reconnect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: c.opts.session.WriteTimeout,
		TLSClientConfig:  c.opts.tlsConf,
	}

	// 先关闭旧连接，防止资源泄漏
	c.Close()

	for {
		select {
		case <-c.opts.ctx.Done():
			return c.opts.ctx.Err()
		default:
		}

		conn, _, err := dialer.DialContext(c.opts.ctx, c.url.String(), nil)
		if err == nil {
			c.retryCount.Store(0)
			c.session = NewSession(c, conn, c.opts.session)
			return nil
		}

		attempt := c.retryCount.Add(1)
		if !c.canRetry() {
			return fmt.Errorf("%v. attempts reached=%d", err, attempt)
		}

		delay := c.calculateBackoff(attempt)
		log.Warnf("Reconnect attempt %d failed, retrying in %s, error: %v", attempt, delay, err)

		select {
		case <-time.After(delay):
		case <-c.opts.ctx.Done():
			return c.opts.ctx.Err()
		}
	}
}

// calculateBackoff 计算指数退避时间
func (c *Client) calculateBackoff(attempt int32) time.Duration {
	backoff := float64(c.opts.retryPolicy.baseDelay) * math.Pow(1.5, float64(attempt))
	backoff = math.Min(backoff, float64(c.opts.retryPolicy.maxDelay))
	return time.Duration(backoff * (0.9 + 0.2*rand.Float64()))
}

// OnSessionOpen 连接成功回调
func (c *Client) OnSessionOpen(sess *Session) {
	if c.opts.connectFunc != nil {
		safeCall(func() { c.opts.connectFunc(sess) })
	}
}

// OnSessionClose 连接关闭回调和重连逻辑
func (c *Client) OnSessionClose(sess *Session) {
	if c.opts.disconnectFunc != nil {
		safeCall(func() { c.opts.disconnectFunc(sess) })
	}

	if c.canRetry() {
		go func() {
			if err := c.Reconnect(); err != nil {
				log.Warnf("reconnect failed: %v", err)
			}
		}()
	}
}

// Request 发送请求消息
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

	payload := proto.Payload{
		Op:      proto.OpRequest,
		Place:   proto.PlaceClient,
		Seq:     seq,
		Code:    0,
		Command: command,
		Body:    data,
	}

	// 缓存序列号对应的命令
	c.reqPool.Store(seq, command)

	// 发送请求
	return c.session.SendPayload(&payload)
}

// DispatchMessage 消息分发，处理不同类型的消息
func (c *Client) DispatchMessage(sess *Session, data []byte) error {
	var p proto.Payload
	if err := gproto.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("unmarshal payload failed: %w", err)
	}

	switch p.Op {
	case proto.OpResponse:
		cmdInterface, loaded := c.reqPool.LoadAndDelete(p.Seq)
		if !loaded {
			log.Warnf("unknown seq %d in reqPool, command=%d", p.Seq, p.Command)
			return nil
		}
		command, ok := cmdInterface.(int32)
		if !ok {
			log.Warnf("invalid command type in reqPool for seq %d", p.Seq)
			return nil
		}

		if handler, exists := c.opts.responseHandler[command]; exists {
			safeCall(func() { handler(p.Body, p.Code) })
		} else {
			log.Warnf("no response handler for command %d", command)
		}

	case proto.OpPush:
		if handler, exists := c.opts.pushHandler[p.Command]; exists {
			safeCall(func() { handler(p.Body) })
		} else {
			log.Warnf("no push handler for command %d", p.Command)
		}

	case proto.OpPing:
		// 收到 Ping，回复 Pong
		if err := sess.SendPayload(&proto.Payload{Op: proto.OpPong}); err != nil {
			log.Warnf("failed to send pong: %v", err)
		}

	case proto.OpPong:
		// 不处理，服务器的 Pong 包

	default:
		log.Warnf("unknown payload Op: %d", p.Op)
	}

	return nil
}

// Close 关闭客户端及其底层连接，清理请求池
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
	c.wg.Wait()
}

// safeCall 用于安全调用回调，避免panic导致崩溃
func safeCall(fn func()) {
	defer xgo.RecoverFromError(nil)
	if fn != nil {
		fn()
	}
}
