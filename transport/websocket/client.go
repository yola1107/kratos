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

	gproto "github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket/proto"
)

var (
	errClosedRequest = errors.New("client: session not established")
	errMaxRetries    = errors.New("client: max retries reached")
	errInvalidURL    = errors.New("client: invalid URL")
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

func WithHeartbeat(d, i, w time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.session.ReadDeadline, o.session.PingInterval, o.session.WriteTimeout = d, i, w
	}
}

func WithSentChanSize(size int) ClientOption {
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

func WithDisconnectFunc(disconnectFunc func(*Session)) ClientOption {
	return func(o *clientOptions) { o.disconnectFunc = disconnectFunc }
}

func WithPushHandler(pushHandler map[int32]PushHandler) ClientOption {
	return func(o *clientOptions) { o.pushHandler = pushHandler }
}

func WithResponseHandler(responseHandler map[int32]ResponseHandler) ClientOption {
	return func(o *clientOptions) { o.responseHandler = responseHandler }
}

func WithRetryPolicy(b, m time.Duration, maxAttempt int32) ClientOption {
	return func(o *clientOptions) {
		o.retryPolicy.baseDelay = b
		o.retryPolicy.maxDelay = m
		o.retryPolicy.maxAttempt = maxAttempt
	}
}

// func WithDiscovery(d registry.Discovery) ClientOption {
//	return func(o *clientOptions) {
//		o.discovery = d
//	}
// }

// func WithSelector(s selector.Selector) ClientOption {
//	return func(o *clientOptions) {
//		o.selector = s
//	}
// }

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
	retryPolicy     *retryPolicy // 重连

	// //服务发现
	// discovery registry.Discovery
}

type retryPolicy struct {
	baseDelay  time.Duration
	maxDelay   time.Duration
	maxAttempt int32
}

// Client is a websocket client.
type Client struct {
	opts       *clientOptions
	url        *url.URL
	seq        int32
	reqPool    sync.Map // seq -> chan *proto.Payload
	session    *Session //
	retryCount atomic.Int32
	wg         *sync.WaitGroup

	// selector selector.Selector
	// resolver *resolver
	// watcher   registry.Watcher
	// endpoints []*url.URL
	// balancer  balancer.Balancer
}

// NewClient returns an websocket client.
func NewClient(ctx context.Context, opts ...ClientOption) (*Client, error) {
	options := &clientOptions{
		ctx:             ctx,
		timeout:         2 * time.Second,
		tlsConf:         nil,
		endpoint:        "ws://0.0.0.0:3102",
		token:           "",
		disconnectFunc:  nil,
		pushHandler:     map[int32]PushHandler{},
		responseHandler: map[int32]ResponseHandler{},
		session: &SessionConfig{
			WriteTimeout: 10 * time.Second,
			PingInterval: 10 * time.Second,
			ReadDeadline: 60 * time.Second,
			SendChanSize: 128,
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

	u, err := parseUrl(options.endpoint, options.tlsConf == nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidURL, err)
	}

	c := &Client{
		opts:       options,
		url:        u,
		seq:        0,
		reqPool:    sync.Map{},
		session:    nil,
		retryCount: atomic.Int32{},
		wg:         &sync.WaitGroup{},
	}

	if err := c.Reconnect(); err != nil {
		return nil, err
	}

	return c, nil
}

func parseUrl(endpoint string, insecure bool) (*url.URL, error) {
	if !strings.Contains(endpoint, "://") {
		if insecure {
			endpoint = "ws://" + endpoint
		} else {
			endpoint = "wss://" + endpoint
		}
	}
	return url.Parse(endpoint)
}

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

func (c *Client) canRetry() bool {
	maxAttempt := c.opts.retryPolicy.maxAttempt
	curr := c.retryCount.Load()

	if maxAttempt < 0 {
		return true // 无限重试
	}
	if maxAttempt == 0 {
		return false // 不允许重试
	}
	return curr < maxAttempt
}

func (c *Client) Reconnect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: c.opts.session.WriteTimeout,
		TLSClientConfig:  c.opts.tlsConf,
	}

	c.Close() // 清理旧连接

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

		curr := c.retryCount.Add(1)
		if !c.canRetry() {
			return fmt.Errorf("%w: %d attempts", errMaxRetries, curr)
		}

		delay := c.calculateBackoff(curr)
		log.Warnf("reconnecting to %q. attempt=%d retrying in %v: %v", c.url, curr, delay, err)

		select {
		case <-time.After(delay):
		case <-c.opts.ctx.Done():
			return c.opts.ctx.Err()
		}
	}
}

// 添加退避时间计算方法
func (c *Client) calculateBackoff(attempt int32) time.Duration {
	backoff := float64(c.opts.retryPolicy.baseDelay) * math.Pow(1.5, float64(attempt))
	backoff = math.Min(backoff, float64(c.opts.retryPolicy.maxDelay))
	return time.Duration(backoff * (0.9 + 0.2*rand.Float64()))
}

func (c *Client) OnSessionOpen(sess *Session) {
	if c.opts.connectFunc != nil {
		c.opts.connectFunc(sess)
	}
}

func (c *Client) OnSessionClose(sess *Session) {
	if c.opts.disconnectFunc != nil {
		c.opts.disconnectFunc(sess)
	}

	if c.canRetry() {
		_ = c.Reconnect()
	}
}

func (c *Client) Request(command int32, msg gproto.Message) error {
	if !c.IsAlive() {
		return errClosedRequest
	}

	seq := atomic.AddInt32(&c.seq, 1)
	if seq >= math.MaxInt32-1 {
		atomic.StoreInt32(&c.seq, 0)
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

	// 存储消息序列号seq
	c.reqPool.Store(seq, command)

	return c.session.SendPayload(&payload)
}

// DispatchMessage 消息分发
func (c *Client) DispatchMessage(sess *Session, data []byte) error {
	var p proto.Payload
	if err := gproto.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("unmarshal error: %w", err)
	}

	switch p.Op {
	case proto.OpResponse:
		if command, loaded := c.reqPool.LoadAndDelete(p.Seq); !loaded {
			log.Error("reqPool seq %d is not exist. command:(%d %d)", p.Seq, p.Command, command)
			return nil
		}
		if handler, ok := c.opts.responseHandler[p.Command]; ok {
			safeCall(func() { handler(p.Body, p.Code) })
		} else {
			log.Warnf("no response handler for command: %d", p.Command)
		}

	case proto.OpPush:
		if handler, ok := c.opts.pushHandler[p.Command]; ok {
			safeCall(func() { handler(p.Body) })
		} else {
			log.Warnf("no push handler for command: %d", p.Command)
		}

	case proto.OpPing:
		_ = sess.SendPayload(&proto.Payload{Op: proto.OpPong})

	case proto.OpPong:
		// server端回pong包. 不处理

	default:
		log.Warnf("unknown payload Op: %d", p.Op)
	}

	return nil
}

func (c *Client) Close() {
	s := c.session
	if s == nil {
		return
	}

	c.session = nil
	s.Close(true)
	c.reqPool.Range(func(key, value any) bool {
		c.reqPool.Delete(key)
		return true
	})
	c.wg.Wait()
}

// safeCall 安全执行回调
func safeCall(fn func()) {
	defer ext.RecoverFromError(nil)
	if fn != nil {
		fn()
	}
}
