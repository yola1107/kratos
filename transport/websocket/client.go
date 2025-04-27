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
	errClosedRequest  = errors.New("client: closed request")
	errRequestTimeout = errors.New("client: request timeout")
	errMaxRetries     = errors.New("client: max retries reached")
	errInvalidURL     = errors.New("client: invalid URL")
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
			maxAttempt: 5,
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

func (c *Client) GetSession() *Session {
	return c.session
}

func (c *Client) CanRetry() bool {
	return c.retryCount.Load() < c.opts.retryPolicy.maxAttempt
}

func (c *Client) Reconnect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: c.opts.session.WriteTimeout,
		TLSClientConfig:  c.opts.tlsConf,
	}

	c.Close() // 清理旧连接

	var attempt int32
	for {
		select {
		case <-c.opts.ctx.Done():
			return c.opts.ctx.Err()
		default:
		}

		conn, _, err := dialer.DialContext(c.opts.ctx, c.url.String(), nil)
		if err == nil {
			c.retryCount.Store(0) // reset retry count on success
			c.session = NewSession(c, conn, c.opts.session)
			// c.onOpen(c.session)
			return nil
		}

		if attempt >= c.opts.retryPolicy.maxAttempt {
			c.retryCount.Store(attempt)
			return fmt.Errorf("%w: %d attempts", errMaxRetries, attempt)
		}

		// 计算退避时间
		delay := c.calculateBackoff(atomic.AddInt32(&attempt, 1))
		log.Warnf("reconnecting to %q. attempt=%d retrying in %v: %v", c.url, attempt, delay, err)

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

func (c *Client) Request(command int32, msg gproto.Message) (*proto.Payload, error) {
	if c.session == nil || c.session.Closed() {
		return nil, errClosedRequest
	}

	seq := atomic.AddInt32(&c.seq, 1)
	if seq >= math.MaxInt32-1 {
		atomic.StoreInt32(&c.seq, 0)
	}
	data, err := buildPayload(proto.OpRequest, command, seq, msg)
	if err != nil {
		return nil, err
	}

	// 注册响应通道
	respChan := make(chan *proto.Payload, 1)
	c.reqPool.Store(seq, respChan)
	defer func() {
		if _, loaded := c.reqPool.LoadAndDelete(seq); loaded {
			close(respChan)
		}
	}()

	// 发送请求
	if err := c.session.Send(data); err != nil {
		return nil, err
	}

	// 等待响应
	select {
	case resp := <-respChan:
		if resp.Code != 0 {
			return resp, fmt.Errorf("error code=%d", resp.Code)
		}
		return resp, nil
	case <-c.opts.ctx.Done():
		return nil, c.opts.ctx.Err()
	case <-time.After(c.opts.session.WriteTimeout):
		return nil, errRequestTimeout
	}
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

	if c.CanRetry() {
		_ = c.Reconnect()
	}
}

// DispatchMessage 消息分发
func (c *Client) DispatchMessage(sess *Session, data []byte) error {
	var p proto.Payload
	if err := gproto.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("unmarshal error: %w", err)
	}

	switch p.Op {
	case proto.OpResponse:
		if ch, loaded := c.reqPool.LoadAndDelete(p.Seq); loaded {
			select {
			case ch.(chan *proto.Payload) <- &p:
			default:
				log.Warnf("response channel blocked or closed for seq %d", p.Seq)
			}
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
	c.clearPendingRequests()
	c.wg.Wait()
	log.Info("client close complete.")
}

func (c *Client) clearPendingRequests() {
	c.reqPool.Range(func(key, value interface{}) bool {
		if ch, ok := value.(chan *proto.Payload); ok {
			select {
			case <-ch: // 尝试消费残留数据
			default:
				close(ch)
			}
		}
		c.reqPool.Delete(key)
		return true
	})
}

// safeCall 安全执行回调
func safeCall(fn func()) {
	defer ext.RecoverFromError(nil)
	if fn != nil {
		fn()
	}
}

// buildPayload 构造协议消息
func buildPayload(op, command, seq int32, msg gproto.Message) ([]byte, error) {
	data, err := gproto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	pl := proto.Payload{
		Op:      op,
		Place:   proto.PlaceClient,
		Seq:     seq,
		Code:    0,
		Command: command,
		Body:    data,
	}
	return gproto.Marshal(&pl)
}
