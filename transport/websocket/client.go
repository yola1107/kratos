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

type ClientOption func(*clientOptions)

func WithTlsConf(tlsConfig *tls.Config) ClientOption {
	return func(o *clientOptions) { o.tlsConf = tlsConfig }
}
func WithTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.sessionConf.timeouts.timeout = timeout }
}
func WithWriteTimeout(write time.Duration) ClientOption {
	return func(o *clientOptions) { o.sessionConf.timeouts.write = write }
}
func WithHeartInterval(heartInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.sessionConf.heartbeat.interval = heartInterval }
}
func WithEndpoint(endpoint string) ClientOption {
	return func(o *clientOptions) { o.endpoint = endpoint }
}
func WithToken(token string) ClientOption {
	return func(o *clientOptions) { o.token = token }
}
func WithPushHandler(pushHandler map[int32]PushHandler) ClientOption {
	return func(o *clientOptions) { o.pushHandler = pushHandler }
}
func WithResponseHandler(responseHandler map[int32]ResponseHandler) ClientOption {
	return func(o *clientOptions) { o.responseHandler = responseHandler }
}
func WithDisconnectFunc(disconnectFunc func()) ClientOption {
	return func(o *clientOptions) { o.disconnectFunc = disconnectFunc }
}

//func WithDiscovery(d registry.Discovery) ClientOption {
//	return func(o *clientOptions) {
//		o.discovery = d
//	}
//}

//func WithSelector(s selector.Selector) ClientOption {
//	return func(o *clientOptions) {
//		o.selector = s
//	}
//}

type clientOptions struct {
	ctx             context.Context
	tlsConf         *tls.Config
	endpoint        string
	token           string
	disconnectFunc  func()
	pushHandler     map[int32]PushHandler
	responseHandler map[int32]ResponseHandler
	sessionConf     *sessionConfig //
	retryPolicy     *retryPolicy   //重连

	////服务发现
	//discovery registry.Discovery
}

type retryPolicy struct {
	//autoRetry bool
	baseDelay  time.Duration
	maxDelay   time.Duration
	maxAttempt int32
}

// Client is a websocket client.
type Client struct {
	opts        clientOptions
	url         *url.URL
	seq         int32
	reqPool     sync.Map // seq -> chan *proto.Payload
	session     *Session //
	retryCount  atomic.Int32
	wg          *sync.WaitGroup
	keepAliveCh chan struct{}
	closed      atomic.Bool

	//selector selector.Selector
	//resolver *resolver
	//watcher   registry.Watcher
	//endpoints []*url.URL
	//balancer  balancer.Balancer
}

// NewClient returns an websocket client.
func NewClient(ctx context.Context, opts ...ClientOption) (*Client, error) {
	options := clientOptions{
		ctx:     ctx,
		tlsConf: nil,

		endpoint:        "ws://0.0.0.0:3102",
		token:           "",
		disconnectFunc:  nil,
		pushHandler:     map[int32]PushHandler{},
		responseHandler: map[int32]ResponseHandler{},
		sessionConf:     defaultSessionConf,
		retryPolicy: &retryPolicy{
			//autoRetry: false,
			baseDelay:  3 * time.Second,
			maxDelay:   15 * time.Second,
			maxAttempt: 5,
		},
	}
	for _, o := range opts {
		o(&options)
	}

	u, err := parseUrl(options.endpoint, options.tlsConf == nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	c := &Client{
		opts:        options,
		url:         u,
		seq:         0,
		reqPool:     sync.Map{},
		session:     nil,
		retryCount:  atomic.Int32{},
		wg:          &sync.WaitGroup{},
		keepAliveCh: make(chan struct{}),
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

func (c *Client) Reconnect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: c.opts.sessionConf.timeouts.write,
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
			c.session = NewSession(c, conn, c.opts.sessionConf)
			go c.keepHeartbeat()
			return nil
		}

		// 计算退避时间
		delay := c.calculateBackoff(atomic.AddInt32(&attempt, 1))
		log.Warnf("Connect failed (attempt %d), retrying in %v: %v", attempt, delay, err)

		select {
		case <-time.After(delay):
		case <-c.opts.ctx.Done():
			return c.opts.ctx.Err()
		default:
			if attempt >= c.opts.retryPolicy.maxAttempt {
				return ErrMaxRetries
			}
		}
	}
}

// 添加退避时间计算方法
func (c *Client) calculateBackoff(attempt int32) time.Duration {
	base := float64(c.opts.retryPolicy.baseDelay)
	max := float64(c.opts.retryPolicy.maxDelay)
	backoff := base * math.Pow(1.5, float64(attempt))
	backoff = math.Min(backoff, max)
	jitter := backoff * 0.1 * rand.Float64()
	return time.Duration(backoff + jitter)
}

func (c *Client) keepHeartbeat() {
	defer RecoverFromError(nil)
	ticker := time.NewTicker(c.opts.sessionConf.heartbeat.interval)
	defer ticker.Stop()

	c.wg.Add(1)
	defer c.wg.Done()

	for {
		select {
		case <-ticker.C:
			// 安全获取 session 实例
			var sess *Session
			if c.session != nil {
				c.session.connMu.Lock()
				sess = c.session
				c.session.connMu.Unlock()
			}

			if sess == nil || sess.Closed() {
				return
			}

			// 检查心跳
			lastActive := sess.LastActive()
			cutoff := time.Now().Add(-c.opts.sessionConf.heartbeat.deadline)
			threshold := time.Now().Add(-c.opts.sessionConf.heartbeat.threshold)

			if lastActive.Before(cutoff) {
				log.Warnf("Session %s heartbeat timeout", sess.id)
				sess.Close(true)
				return
			} else if lastActive.Before(threshold) {
				log.Infof("Session %s send keepalive ping", sess.id)
				if err := sess.Send(mustMarshal(&proto.Payload{Type: int32(proto.Ping)})); err != nil {
					log.Errorf("Send ping failed: %v", err)
				}
			}

		case <-c.keepAliveCh:
			return
		case <-c.opts.ctx.Done():
			return
		}
	}
}

func (c *Client) Request(ops int32, msg gproto.Message) (*proto.Payload, error) {
	if c.session.Closed() {
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
	case <-c.opts.ctx.Done():
		return nil, c.opts.ctx.Err()
	case <-time.After(c.opts.sessionConf.timeouts.write):
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
		if handler, ok := c.opts.responseHandler[p.Op]; ok {
			safeCall(func() { handler(p.Body, p.Code) })
		} else {
			log.Warnf("no response handler for op: %d", p.Op)
		}

	case int32(proto.Push):
		var body proto.Body
		if err := gproto.Unmarshal(p.Body, &body); err != nil {
			return fmt.Errorf("unmarshal push body error: %w", err)
		}
		if handler, ok := c.opts.pushHandler[body.Ops]; ok {
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
	if !c.closed.CompareAndSwap(false, true) {
		return
	}
	close(c.keepAliveCh)

	if c.session != nil {
		c.session.Close(true)
		c.session = nil
	}

	c.clearPendingRequests()
	c.wg.Wait()
	log.Info("Client shutdown complete")
}

func (c *Client) Closed() bool {
	return c.session == nil || c.session.Closed()
}

func (c *Client) onClose(*Session) {
	if c.opts.disconnectFunc != nil {
		c.opts.disconnectFunc()
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

func RecoverFromError(logFn func(err any)) {
	if r := recover(); r != nil {
		if logFn != nil {
			logFn(r)
		} else {
			log.Errorf("panic recovered: %v\n%s", r, debug.Stack())
		}
	}
}
