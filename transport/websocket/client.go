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
	return func(o *clientOptions) { o.timeout = timeout }
}
func WithWriteTimeout(writeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.writeTimeout = writeTimeout }
}
func WithReadTimeout(readTimeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.readTimeout = readTimeout }
}
func WithResponseTimeout(responseTimeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.responseTimeout = responseTimeout }
}
func WithHeartInterval(heartInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartInterval = heartInterval }
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
		o.reconnectBaseDelay = baseDelay
		o.reconnectMaxDelay = maxDelay
		o.maxReconnectAttempt = attemptCnt
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
	ctx             context.Context
	tlsConf         *tls.Config
	timeout         time.Duration
	writeTimeout    time.Duration
	readTimeout     time.Duration
	responseTimeout time.Duration
	heartInterval   time.Duration
	endpoint        string
	token           string
	disconnectFunc  func()
	pushHandler     map[int32]PushHandler
	responseHandler map[int32]ResponseHandler
	stateFunc       func(connected bool)

	//重连
	reconnectBaseDelay  time.Duration
	reconnectMaxDelay   time.Duration
	maxReconnectAttempt int32

	////服务发现
	//discovery registry.Discovery
}

// Client is an websocket client.
type Client struct {
	opts clientOptions

	url       *url.URL
	seq       int32
	reqPool   sync.Map // seq -> chan *proto.Payload
	conn      *websocket.Conn
	connMutex sync.RWMutex
	writeChan chan *proto.Payload
	closeChan chan struct{}
	wg        sync.WaitGroup

	isClosed   atomic.Bool
	retryCount atomic.Int32

	//selector selector.Selector
	//resolver *resolver
	//watcher   registry.Watcher
	//endpoints []*url.URL
	//balancer  balancer.Balancer
}

// NewClient returns an websocket client.
func NewClient(ctx context.Context, opts ...ClientOption) (*Client, error) {
	options := clientOptions{
		ctx:                 ctx,
		tlsConf:             nil,
		timeout:             1 * time.Second,
		writeTimeout:        15 * time.Second,
		readTimeout:         60 * time.Second,
		responseTimeout:     15 * time.Second,
		heartInterval:       10 * time.Second,
		endpoint:            "ws://0.0.0.0:3102",
		token:               "",
		disconnectFunc:      nil,
		pushHandler:         map[int32]PushHandler{},
		responseHandler:     map[int32]ResponseHandler{},
		reconnectBaseDelay:  3 * time.Second,
		reconnectMaxDelay:   15 * time.Second,
		maxReconnectAttempt: 5,
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
		writeChan:  make(chan *proto.Payload, 100),
		closeChan:  make(chan struct{}),
		wg:         sync.WaitGroup{},
		isClosed:   atomic.Bool{},
		retryCount: atomic.Int32{},
	}

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
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if c.isClosed.Load() {
		return ErrClientClosed
	}

	// 关闭现有连接
	if c.conn != nil {
		_ = c.conn.Close()
	}

	// 使用带超时和TLS的Dialer
	dialer := websocket.Dialer{
		HandshakeTimeout: c.opts.writeTimeout,
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
	c.notifyState(true)

	// 启动处理协程
	c.wg.Add(3)
	go c.readPump()
	go c.writePump()
	go c.heartbeatLoop()

	return nil
}

func (c *Client) Request(ops int32, msg gproto.Message) (*proto.Payload, error) {

	if c.isClosed.Load() {
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
		c.reqPool.Delete(seq)
		close(respChan)
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
	case <-time.After(c.opts.responseTimeout):
		return nil, ErrRequestTimeout
	}
}

// send 发送消息到写入队列
func (c *Client) send(p *proto.Payload) error {
	if c.isClosed.Load() {
		return ErrClientClosed
	}

	select {
	case c.writeChan <- p:
		return nil
	case <-time.After(c.opts.writeTimeout):
		return ErrSendTimeout
	case <-c.closeChan:
		return ErrClientClosed
	}
}

// heartbeatLoop 心跳维持协程
func (c *Client) heartbeatLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.opts.heartInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 发送ping
			if err := c.send(&proto.Payload{Type: int32(proto.Ping)}); err != nil {
				log.Errorf("send ping failed: %v", err)
			}
		case <-c.closeChan:
			return
		}
	}
}

// writePump 写入协程
func (c *Client) writePump() {
	defer c.wg.Done()

	for {
		select {
		case p := <-c.writeChan:
			data, err := gproto.Marshal(p)
			if err != nil {
				log.Errorf("marshal error: %v", err)
				continue
			}

			c.connMutex.Lock()
			err = c.conn.SetWriteDeadline(time.Now().Add(c.opts.writeTimeout))
			if err == nil {
				err = c.conn.WriteMessage(websocket.BinaryMessage, data)
			}
			c.connMutex.Unlock()

			if err != nil {
				log.Warnf("write error: %v", err)
				c.safeReconnect()
				return
			}

		case <-c.closeChan:
			return
		}
	}
}

// readPump 读取协程
func (c *Client) readPump() {
	defer c.wg.Done()
	defer c.Close()

	_ = c.conn.SetReadDeadline(time.Now().Add(c.opts.readTimeout))

	for {
		msgType, data, err := c.conn.ReadMessage()
		if err != nil {
			if !c.isClosed.Load() && !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				log.Warnf("read error: %v", err)
				c.safeReconnect()
			}
			return
		}

		_ = c.conn.SetReadDeadline(time.Now().Add(c.opts.readTimeout))

		switch msgType {
		case websocket.BinaryMessage:
			if err := c.handleMessage(data); err != nil {
				log.Errorf("handle message error: %v", err)
			}
		case websocket.PingMessage:
			_ = c.conn.WriteMessage(websocket.PongMessage, nil)
		case websocket.CloseMessage:
			log.Info("server initiated close")
			return
		}
	}
}

// handleMessage 处理接收到的消息
func (c *Client) handleMessage(data []byte) error {
	var p proto.Payload
	if err := gproto.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("unmarshal error: %w", err)
	}

	switch p.Type {
	case int32(proto.Response):
		if ch, ok := c.reqPool.Load(p.Seq); ok {
			select {
			case ch.(chan *proto.Payload) <- &p:
			default:
				log.Warnf("response channel full for seq %d", p.Seq)
			}
		}
		if handler, ok := c.opts.responseHandler[p.Op]; ok {
			safeCall(func() { handler(p.Body, p.Code) })
		} else {
			log.Warnf("Unkonwn Ops(%+v). response", p.Type)
		}

	case int32(proto.Push):
		var body proto.Body
		if err := gproto.Unmarshal(p.Body, &body); err != nil {
			return fmt.Errorf("unmarshal push body error: %w", err)
		}
		if handler, ok := c.opts.pushHandler[body.Ops]; ok {
			safeCall(func() { handler(body.Data) })
		} else {
			log.Warnf("Unkonwn Ops(%+v). push", body.Ops)
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
	if c.isClosed.Load() || c.retryCount.Load() >= c.opts.maxReconnectAttempt {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("reconnect panic. %+v", r)
			}
		}()

		for i := int32(1); i <= c.opts.maxReconnectAttempt; i++ {
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
			case <-c.closeChan: //close(c.closeChan)时触发
				return
			}
		}

		log.Error("max reconnect attempts reached")
		c.Close()
	}()
}

// calculateBackoff 计算退避时间
func (c *Client) calculateBackoff(attempt int32) time.Duration {
	base := float64(c.opts.reconnectBaseDelay)
	max := float64(c.opts.reconnectMaxDelay)
	jitter := 0.2 * rand.Float64()

	delay := time.Duration(base * math.Pow(1.5, float64(attempt-1)) * (1 + jitter))
	return time.Duration(math.Min(float64(delay), max))
}

// Close 安全关闭连接
func (c *Client) Close() {
	if !c.isClosed.CompareAndSwap(false, true) {
		return
	}

	close(c.closeChan)

	// 有序关闭
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

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
	return c.isClosed.Load()
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

// safeCall 安全执行回调
func safeCall(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("handler panic: %v", r)
		}
	}()
	fn()
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
