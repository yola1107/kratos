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
	ErrSendTimeout    = errors.New("send timeout")
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
	return func(o *clientOptions) { o.timeouts.timeout = timeout }
}
func WithWriteTimeout(write time.Duration) ClientOption {
	return func(o *clientOptions) { o.timeouts.write = write }
}
func WithReadTimeout(read time.Duration) ClientOption {
	return func(o *clientOptions) { o.timeouts.read = read }
}
func WithHeartInterval(heartInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartbeat.interval = heartInterval }
}
func WithEndpoint(endpoint string) ClientOption {
	return func(o *clientOptions) { o.endpoint = endpoint }
}
func WithToken(token string) ClientOption {
	return func(o *clientOptions) { o.token = token }
}
func WithDisconnectFunc(disconnectFunc func()) ClientOption {
	return func(o *clientOptions) { o.disconnectFunc = disconnectFunc }
}
func WithPushHandler(pushHandler map[int32]PushHandler) ClientOption {
	return func(o *clientOptions) { o.pushHandler = pushHandler }
}
func WithResponseHandler(responseHandler map[int32]ResponseHandler) ClientOption {
	return func(o *clientOptions) { o.responseHandler = responseHandler }
}
func WithReconnect(baseDelay, maxDelay time.Duration, attemptCnt int32) ClientOption {
	return func(o *clientOptions) {
		o.retryPolicy.baseDelay = baseDelay
		o.retryPolicy.maxDelay = maxDelay
		o.retryPolicy.maxAttempt = attemptCnt
	}
}
func WithStateFunc(f func(bool)) ClientOption {
	return func(o *clientOptions) { o.stateFunc = f }
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
	ctx     context.Context
	tlsConf *tls.Config

	endpoint        string
	token           string
	disconnectFunc  func()
	pushHandler     map[int32]PushHandler
	responseHandler map[int32]ResponseHandler
	stateFunc       func(connected bool)

	timeouts    clientTimeouts //
	heartbeat   clientHeartbeat
	retryPolicy retryPolicy //重连

	////服务发现
	//discovery registry.Discovery
}
type clientTimeouts struct {
	timeout time.Duration
	write   time.Duration
	read    time.Duration
}
type clientHeartbeat struct {
	interval  time.Duration
	deadline  time.Duration
	threshold time.Duration
}
type retryPolicy struct {
	baseDelay  time.Duration
	maxDelay   time.Duration
	maxAttempt int32
}

// Client is a websocket client.
type Client struct {
	opts clientOptions

	url      *url.URL
	seq      int32
	reqPool  sync.Map // seq -> chan *proto.Payload
	conn     *websocket.Conn
	connMu   sync.RWMutex
	sendChan chan *proto.Payload
	closeCh  chan struct{}
	wg       sync.WaitGroup

	closed     atomic.Bool
	retryCount atomic.Int32

	lastActive atomic.Value // time.Time

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
		timeouts: clientTimeouts{
			timeout: 1 * time.Second,
			write:   15 * time.Second,
			read:    60 * time.Second,
		},
		heartbeat: clientHeartbeat{
			interval:  10 * time.Second,
			deadline:  60 * time.Second,
			threshold: 30 * time.Second,
		},
		retryPolicy: retryPolicy{
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
		opts:       options,
		url:        u,
		seq:        0,
		reqPool:    sync.Map{},
		conn:       nil,
		sendChan:   make(chan *proto.Payload, 100),
		closeCh:    make(chan struct{}),
		wg:         sync.WaitGroup{},
		closed:     atomic.Bool{},
		retryCount: atomic.Int32{},
	}
	c.lastActive.Store(time.Now())

	if err := c.establishConnection(); err != nil {
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

// establishConnection 内部连接方法
func (c *Client) establishConnection() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.Closed() {
		return ErrClientClosed
	}

	// 关闭现有连接
	if c.conn != nil {
		_ = c.conn.Close()
	}

	// 使用带超时和TLS的Dialer
	dialer := websocket.Dialer{
		HandshakeTimeout: c.opts.timeouts.write,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			//InsecureSkipVerify: true, // 开发用，跳过证书校验
		},
	}

	conn, _, err := dialer.Dial(c.url.String(), nil)
	if err != nil {
		c.notifyState(false)
		return fmt.Errorf("connection failed: %w", err)
	}

	c.conn = conn
	c.setLastActive()
	c.notifyState(true)

	// 启动处理协程
	c.wg.Add(3)
	go c.readPump()
	go c.writePump()
	go c.heartbeat()

	return nil
}

func (c *Client) Request(ops int32, msg gproto.Message) (*proto.Payload, error) {

	if c.closed.Load() {
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
	if err := c.send(p); err != nil {
		return nil, err
	}

	// 等待响应
	select {
	case resp := <-respChan:
		return resp, nil
	case <-c.opts.ctx.Done():
		return nil, c.opts.ctx.Err()
	case <-time.After(c.opts.timeouts.write - 1):
		return nil, ErrRequestTimeout
	}
}

// send 发送消息到写入队列
func (c *Client) send(p *proto.Payload) error {
	if c.Closed() {
		return ErrClientClosed
	}

	select {
	case c.sendChan <- p:
		return nil
	case <-c.closeCh:
		return ErrClientClosed
	case <-time.After(c.opts.timeouts.write):
		return ErrSendTimeout
	}
}

// heartbeat 心跳维持协程
func (c *Client) heartbeat() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.opts.heartbeat.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 发送ping
			if err := c.send(&proto.Payload{Type: int32(proto.Ping)}); err != nil {
				log.Errorf("send ping failed: %v", err)
			}

			cutoff := time.Now().Add(-1 * c.opts.heartbeat.deadline)
			threshold := time.Now().Add(-1 * c.opts.heartbeat.threshold)
			if c.LastActive().Before(cutoff) {
				log.Warnf("heartbeat dead line.")
				c.safeReconnect()
			} else if c.LastActive().Before(threshold) {
				log.Warnf("heartbeat threshold. send ping")
				c.connMu.Lock()
				_ = c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(c.opts.timeouts.write))
				c.connMu.Unlock()
			}

		case <-c.closeCh:
			return
		}
	}
}

// writePump 写入协程
func (c *Client) writePump() {
	defer c.wg.Done()

	for {
		select {
		case p := <-c.sendChan:
			data, err := gproto.Marshal(p)
			if err != nil {
				log.Errorf("marshal error: %v", err)
				continue
			}

			c.connMu.Lock()
			err = c.writeMessageLocked(data)
			c.connMu.Unlock()

			if err != nil {
				log.Warnf("write error: %v", err)
				c.safeReconnect()
				return
			}

		case <-c.closeCh:
			return
		}
	}
}

// readPump 读取协程
func (c *Client) readPump() {
	defer c.wg.Done()
	defer c.Close()

	for {
		// 每次读取前设置截止时间
		c.connMu.Lock()
		err := c.conn.SetReadDeadline(time.Now().Add(c.opts.timeouts.read))
		c.connMu.Unlock()
		if err != nil {
			log.Errorf("set read deadline error: %v", err)
			return
		}

		messageType, data, err := c.conn.ReadMessage()
		if err != nil {
			if c.closed.Load() {
				return
			}
			// 错误分类处理
			switch {
			case websocket.IsCloseError(err, websocket.CloseNormalClosure):
				log.Info("server initiated normal closure")
				return
			case websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure):
				log.Warnf("unexpected closure: %v, reconnecting...", err)
				c.safeReconnect()
			default:
				log.Errorf("critical read error: %v", err)
				c.Close()
			}
			return
		}

		c.setLastActive()

		switch messageType {
		case websocket.BinaryMessage:
			if err := c.dispatchMessage(data); err != nil {
				log.Errorf("handle message error: %v", err)
			}

		case websocket.PingMessage:
			c.connMu.Lock()
			_ = c.conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(c.opts.timeouts.write))
			c.connMu.Unlock()

		case websocket.PongMessage:

		case websocket.CloseMessage:
			return

		default:
			log.Warnf("unsupported message type: %d", messageType)
		}
	}
}

// dispatchMessage 处理接收到的消息
func (c *Client) dispatchMessage(data []byte) error {
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
			log.Warnf("handle func is not exist with resp Ops(%+v)", p.Op)
		}

	case int32(proto.Push):
		var body proto.Body
		if err := gproto.Unmarshal(p.Body, &body); err != nil {
			return fmt.Errorf("unmarshal push body error: %w", err)
		}
		if handler, ok := c.opts.pushHandler[body.Ops]; ok {
			safeCall(func() { handler(body.Data) })
		} else {
			log.Warnf("handle func is not exist with push Ops(%+v)", body.Ops)
		}

	case int32(proto.Ping):
		// server端通知client端发送心跳包
		_ = c.send(&proto.Payload{Type: int32(proto.Ping)})

	case int32(proto.Pong):
		// server端回pong包. 不处理

	default:
		log.Warnf("Unknown message type(%+v). ", p.Type)
	}

	return nil
}

// safeReconnect 安全重连逻辑
func (c *Client) safeReconnect() {
	if c.closed.Load() || c.retryCount.Load() >= c.opts.retryPolicy.maxAttempt {
		return
	}

	go func() {
		defer RecoverFromError(nil)

		for i := int32(1); i <= c.opts.retryPolicy.maxAttempt; i++ {
			delay := c.calculateBackoff(i)
			log.Warnf("reconnecting attempt %d, delay %v", i, delay)
			select {
			case <-time.After(delay):
				if err := c.establishConnection(); err == nil {
					c.retryCount.Store(0)
					return
				} else {
					c.retryCount.Store(i)
					log.Warnf("reconnect attempt %d failed: %v", i, err)
				}
			case <-c.closeCh: //close(c.closeCh)时触发
				return
			}
		}

		log.Error("max reconnect attempts reached")
		c.Close()
	}()
}

// calculateBackoff 计算退避时间
func (c *Client) calculateBackoff(attempt int32) time.Duration {
	base := float64(c.opts.retryPolicy.baseDelay)
	max := float64(c.opts.retryPolicy.maxDelay)
	jitter := 0.2 * rand.Float64()

	delay := time.Duration(base * math.Pow(1.5, float64(attempt-1)) * (1 + jitter))
	return time.Duration(math.Min(float64(delay), max))
}

// setLastActive 设置最后活跃时间
func (c *Client) setLastActive() {
	c.lastActive.Store(time.Now())
}

// LastActive 返回最后活跃时间
func (c *Client) LastActive() time.Time {
	return c.lastActive.Load().(time.Time)
}

// Close 安全关闭连接
func (c *Client) Close() {
	if !c.closed.CompareAndSwap(false, true) {
		return
	}

	close(c.closeCh)

	// 有序关闭
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = c.conn.Close()
	}

	// 清理所有等待中的请求
	c.clearPendingRequests()

	c.wg.Wait()
	c.notifyState(false)

	if c.opts.disconnectFunc != nil {
		safeCall(c.opts.disconnectFunc)
	}

	log.Infof("websocket client closed")
}

func (c *Client) Closed() bool {
	return c.closed.Load()
}

// notifyState 通知状态变化
func (c *Client) notifyState(connected bool) {
	if c.opts.stateFunc != nil {
		safeCall(func() { c.opts.stateFunc(connected) })
	}
}

// 清理所有等待响应的请求，防止 goroutine 泄露或 deadlock
func (c *Client) clearPendingRequests() {
	c.reqPool.Range(func(key, value interface{}) bool {
		ch := value.(chan *proto.Payload)
		close(ch)
		c.reqPool.Delete(key)
		return true
	})
}

func (c *Client) writeMessageLocked(data []byte) error {
	if c.conn == nil {
		return errSessionClosed
	}
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.opts.timeouts.write)); err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.BinaryMessage, data)
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

func RecoverFromError(cb func()) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v:%s\n", e, debug.Stack())
		if cb != nil {
			cb()
		}
	}
}
