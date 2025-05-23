package websocket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gproto "github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket/proto"
)

var (
	ErrClientClosed   = errors.New("websocket client is closed")
	ErrRequestTimeout = errors.New("request timeout")
	ErrMaxRetries     = errors.New("max retries reached")
	ErrInvalidURL     = errors.New("invalid URL")
)

type PushHandler func(data []byte)
type ResponseHandler func(data []byte, code int32)

type ClientOption func(*clientConfig)

func WithTlsConf(tlsConfig *tls.Config) ClientOption {
	return func(o *clientConfig) { o.tlsConf = tlsConfig }
}

func WithTimeout(timeout time.Duration) ClientOption {
	return func(o *clientConfig) { o.session.Timeout = timeout }
}

func WithSessionConfig(c *SessionConfig) ClientOption {
	return func(o *clientConfig) { o.session = c }
}

func WithEndpoint(endpoint string) ClientOption {
	return func(o *clientConfig) { o.endpoint = endpoint }
}

func WithToken(token string) ClientOption {
	return func(o *clientConfig) { o.token = token }
}

func WithConnectFunc(fn func(*Session)) ClientOption {
	return func(o *clientConfig) { o.connectFunc = fn }
}

func WithDisconnectFunc(disconnectFunc func(*Session)) ClientOption {
	return func(o *clientConfig) { o.disconnectFunc = disconnectFunc }
}

func WithPushHandler(pushHandler map[int32]PushHandler) ClientOption {
	return func(o *clientConfig) { o.pushHandler = pushHandler }
}

func WithResponseHandler(responseHandler map[int32]ResponseHandler) ClientOption {
	return func(o *clientConfig) { o.responseHandler = responseHandler }
}

//func WithDiscovery(d registry.Discovery) ClientOption {
//	return func(o *clientConfig) {
//		o.discovery = d
//	}
//}

//func WithSelector(s selector.Selector) ClientOption {
//	return func(o *clientConfig) {
//		o.selector = s
//	}
//}

type clientConfig struct {
	ctx             context.Context
	tlsConf         *tls.Config
	endpoint        string
	token           string
	connectFunc     func(*Session)
	disconnectFunc  func(*Session)
	pushHandler     map[int32]PushHandler
	responseHandler map[int32]ResponseHandler
	session         *SessionConfig
	retryPolicy     *retryPolicy //重连

	////服务发现
	//discovery registry.Discovery
}

type retryPolicy struct {
	baseDelay  time.Duration
	maxDelay   time.Duration
	maxAttempt int32
}

// Client is a websocket client.
type Client struct {
	config     *clientConfig
	url        *url.URL
	seq        int32
	reqPool    sync.Map // seq -> chan *proto.Payload
	session    *Session //
	retryCount atomic.Int32
	wg         *sync.WaitGroup

	//selector selector.Selector
	//resolver *resolver
	//watcher   registry.Watcher
	//endpoints []*url.URL
	//balancer  balancer.Balancer
}

// NewClient returns an websocket client.
func NewClient(ctx context.Context, opts ...ClientOption) (*Client, error) {
	cfg := &clientConfig{
		ctx:             ctx,
		tlsConf:         nil,
		endpoint:        "ws://0.0.0.0:3102",
		token:           "",
		disconnectFunc:  nil,
		pushHandler:     map[int32]PushHandler{},
		responseHandler: map[int32]ResponseHandler{},
		session: &SessionConfig{
			Timeout:      1 * time.Second,
			WriteTimeout: 10 * time.Second,
			Interval:     10 * time.Second,
			Deadline:     60 * time.Second,
			Threshold:    30 * time.Second,
			RateLimit:    100, // 每秒消息数,
			BurstLimit:   10,  // 突发消息数,
			SendChanSize: 256,
		},
		retryPolicy: &retryPolicy{
			baseDelay:  3 * time.Second,
			maxDelay:   15 * time.Second,
			maxAttempt: 5,
		},
	}
	for _, o := range opts {
		o(cfg)
	}

	u, err := parseUrl(cfg.endpoint, cfg.tlsConf == nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	c := &Client{
		config:     cfg,
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
	return c.retryCount.Load() < c.config.retryPolicy.maxAttempt
}

func (c *Client) Reconnect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: c.config.session.WriteTimeout,
		TLSClientConfig:  c.config.tlsConf,
	}

	c.Close() // 清理旧连接

	var attempt int32
	for {
		select {
		case <-c.config.ctx.Done():
			return c.config.ctx.Err()
		default:
		}

		conn, _, err := dialer.DialContext(c.config.ctx, c.url.String(), nil)
		if err == nil {
			c.session = NewSession(c, conn, c.config.session)
			c.retryCount.Store(0) // reset retry count on success
			if c.config.connectFunc != nil {
				c.config.connectFunc(c.session)
			}
			return nil
		}

		if attempt >= c.config.retryPolicy.maxAttempt {
			c.retryCount.Store(attempt)
			return fmt.Errorf("%w: %d attempts", ErrMaxRetries, attempt)
		}

		// 计算退避时间
		delay := c.calculateBackoff(atomic.AddInt32(&attempt, 1))
		log.Warnf("reconnecting to %s. attempt=%d retrying in %v: %v", c.url, attempt, delay, err)

		select {
		case <-time.After(delay):
		case <-c.config.ctx.Done():
			return c.config.ctx.Err()
		}
	}
}

// 添加退避时间计算方法
func (c *Client) calculateBackoff(attempt int32) time.Duration {
	backoff := float64(c.config.retryPolicy.baseDelay) * math.Pow(1.5, float64(attempt))
	backoff = math.Min(backoff, float64(c.config.retryPolicy.maxDelay))
	return time.Duration(backoff * (0.9 + 0.2*rand.Float64()))
}

func (c *Client) Request(ops int32, msg gproto.Message) (*proto.Payload, error) {
	if c.session == nil || c.session.Closed() {
		return nil, ErrClientClosed
	}

	seq := atomic.AddInt32(&c.seq, 1)
	p, err := buildPayload(ops, int32(proto.Request), msg)
	if err != nil {
		return nil, err
	}
	p.Seq = seq

	// 注册响应通道
	respChan := make(chan *proto.Payload, 1)
	c.reqPool.Store(seq, respChan)
	defer func() {
		if _, loaded := c.reqPool.LoadAndDelete(seq); loaded {
			close(respChan)
		}
	}()

	// 发送请求
	if err := c.session.Send(mustMarshal(p)); err != nil {
		return nil, err
	}

	// 等待响应
	select {
	case resp := <-respChan:
		return resp, nil
	case <-c.config.ctx.Done():
		return nil, c.config.ctx.Err()
	case <-time.After(c.config.session.WriteTimeout):
		return nil, ErrRequestTimeout
	}
}

// dispatch 消息分发
func (c *Client) dispatch(sess *Session, data []byte) error {
	var p proto.Payload
	if err := gproto.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("unmarshal error: %w", err)
	}

	switch p.Type {
	case int32(proto.Response):
		if ch, loaded := c.reqPool.LoadAndDelete(p.Seq); loaded {
			select {
			case ch.(chan *proto.Payload) <- &p:
			default:
				log.Warnf("response channel closed for seq %d", p.Seq)
			}
		}
		if handler, ok := c.config.responseHandler[p.Op]; ok {
			safeCall(func() { handler(p.Body, p.Code) })
		} else {
			log.Warnf("no response handler for op: %d", p.Op)
		}

	case int32(proto.Push):
		var body proto.Body
		if err := gproto.Unmarshal(p.Body, &body); err != nil {
			return fmt.Errorf("unmarshal push body error: %w", err)
		}
		if handler, ok := c.config.pushHandler[body.Ops]; ok {
			safeCall(func() { handler(body.Data) })
		} else {
			log.Warnf("no push handler for ops: %d", body.Ops)
		}

	case int32(proto.Ping):
		// server端通知client端发送心跳包
		_ = sess.Send(mustMarshal(&proto.Payload{Type: int32(proto.Ping)}))

	case int32(proto.Pong):
		// server端回pong包. 不处理

	default:
		log.Warnf("unknown payload type: %d", p.Type)
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

func (c *Client) onClose(sess *Session) {
	if c.config.disconnectFunc != nil {
		c.config.disconnectFunc(sess)
	}

	if c.CanRetry() {
		_ = c.Reconnect()
	}
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
	defer RecoverFromError(nil)
	if fn != nil {
		fn()
	}
}

// buildPayload 构造协议消息
func buildPayload(ops int32, typ int32, msg gproto.Message) (*proto.Payload, error) {
	data, err := gproto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	body := &proto.Body{Ops: ops, Data: data}
	bodyData, err := gproto.Marshal(body)
	if err != nil {
		return nil, err
	}
	return &proto.Payload{
		Op:   ops,
		Type: typ,
		Body: bodyData,
	}, nil
}

func RecoverFromError(cb func(err any)) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v:%s\n", e, debug.Stack())
		if cb != nil {
			cb(e)
		}
	}
}
