package websocket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sync"
	"time"

	gproto "github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/yola1107/kratos/v2/internal/endpoint"
	"github.com/yola1107/kratos/v2/internal/host"
	"github.com/yola1107/kratos/v2/internal/matcher"
	"github.com/yola1107/kratos/v2/library/task"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware"
	"github.com/yola1107/kratos/v2/transport"
	"github.com/yola1107/kratos/v2/transport/websocket/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	_ transport.Server     = (*Server)(nil)
	_ transport.Endpointer = (*Server)(nil)
)

// ServerOption is a Websocket server option.
type ServerOption func(*Server)

func Network(network string) ServerOption {
	return func(s *Server) { s.opts.network = network }
}
func Address(addr string) ServerOption {
	return func(s *Server) { s.opts.address = addr }
}
func Endpoint(u *url.URL) ServerOption {
	return func(s *Server) { s.opts.endpoint = u }
}
func Timeout(d time.Duration) ServerOption {
	return func(s *Server) { s.opts.timeout = d }
}
func Middleware(m ...middleware.Middleware) ServerOption {
	return func(s *Server) { s.middleware.Use(m...) }
}
func HeartInterval(d time.Duration) ServerOption {
	return func(s *Server) { s.opts.heartInterval = d }
}
func HeartDeadline(d time.Duration) ServerOption {
	return func(s *Server) { s.opts.heartDeadline = d }
}
func HeartThreshold(d time.Duration) ServerOption {
	return func(s *Server) { s.opts.heartThreshold = d }
}
func JobsCnt(cnt int) ServerOption {
	return func(s *Server) { s.opts.maxJobsCnt = cnt }
}
func MaxConnections(max int32) ServerOption {
	return func(s *Server) { s.opts.maxConnections = max }
}
func OnOpenFunc(f func(*Session)) ServerOption {
	return func(s *Server) { s.OnOpenFunc = f }
}
func OnCloseFunc(f func(*Session)) ServerOption {
	return func(s *Server) { s.OnCloseFunc = f }
}

type serverOptions struct {
	network          string
	address          string
	endpoint         *url.URL
	timeout          time.Duration
	lis              net.Listener
	tlsConf          *tls.Config
	writeTimeout     time.Duration
	readTimeout      time.Duration
	heartInterval    time.Duration
	heartDeadline    time.Duration
	heartThreshold   time.Duration
	maxJobsCnt       int
	maxConnections   int32
	closeGracePeriod time.Duration //= 2 * time.Second
	sendChannelSize  time.Duration //= 256
	shutdownTimeout  time.Duration //= 5 * time.Second
	maxMessageSize   int64         //= 10 * 1024 * 1024 // 10MB
	rateLimit        int           //= 1000             // 每秒消息数
	burstLimit       int           //= 500              // 突发消息数
}

// Server is a Websocket server wrapper.
type Server struct {
	*http.Server // 内嵌标准HTTP服务器

	err        error
	opts       serverOptions       // 配置选项
	middleware matcher.Matcher     // 中间件
	upgrader   *websocket.Upgrader // WebSocket升级器
	sessionMgr *SessionManager     // 会话管理

	register   chan *Session // 注册通道
	unregister chan *Session // 注销通道

	handlers []UnaryServerInterceptor // 拦截器链
	m        *service                 // 注册的服务

	loop *task.Loop // 任务处理池

	OnOpenFunc  func(*Session) // 连接建立回调
	OnCloseFunc func(*Session) // 连接关闭回调
}

// NewServer creates a Websocket server by options.
func NewServer(opts ...ServerOption) *Server {
	s := &Server{
		opts: serverOptions{
			network:          "tcp",
			address:          ":0",
			timeout:          1 * time.Second,
			lis:              nil,
			tlsConf:          nil,
			writeTimeout:     15 * time.Second,
			readTimeout:      30 * time.Second,
			heartInterval:    10 * time.Second,
			heartDeadline:    60 * time.Second,
			heartThreshold:   30 * time.Second,
			maxJobsCnt:       10000,
			maxConnections:   10000,
			closeGracePeriod: 2 * time.Second,
			sendChannelSize:  256,
			shutdownTimeout:  5 * time.Second,
			maxMessageSize:   10 * 1024 * 1024, // 10MB
			rateLimit:        1000,             // 每秒消息数
			burstLimit:       500,              // 突发消息数
		},
		err:        nil,
		middleware: matcher.New(),
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		sessionMgr: NewSessionManager(),
		register:   make(chan *Session, 100),
		unregister: make(chan *Session, 100),
	}

	for _, o := range opts {
		o(s)
	}

	s.loop = task.NewLoop(s.opts.maxJobsCnt)
	s.loop.Start()

	s.Use(s.recovery())
	return s
}

func (s *Server) Use(handlers ...UnaryServerInterceptor) *Server {
	finalSize := len(s.handlers) + len(handlers)
	if finalSize >= math.MaxInt8/2 {
		panic("websocket: server use too many handlers")
	}
	mergedHandlers := make([]UnaryServerInterceptor, finalSize)
	copy(mergedHandlers, s.handlers)
	copy(mergedHandlers[len(s.handlers):], handlers)
	s.handlers = mergedHandlers
	return s
}

func (s *Server) Endpoint() (*url.URL, error) {
	if err := s.listenAndEndpoint(); err != nil {
		return nil, err
	}
	return s.opts.endpoint, nil
}

func (s *Server) GetLoop() *task.Loop {
	return s.loop
}

func (s *Server) listenAndEndpoint() error {
	if s.opts.lis == nil {
		lis, err := net.Listen(s.opts.network, s.opts.address)
		if err != nil {
			s.err = err
			return err
		}
		s.opts.lis = lis
	}
	if s.opts.endpoint == nil {
		addr, err := host.Extract(s.opts.address, s.opts.lis)
		if err != nil {
			s.err = err
			return err
		}
		s.opts.endpoint = endpoint.NewEndpoint(endpoint.Scheme("ws", s.opts.tlsConf != nil), addr)
	}
	return s.err
}

// Start start the Websocket server.
func (s *Server) Start(ctx context.Context) error {
	if err := s.listenAndEndpoint(); err != nil {
		return err
	}

	go s.serve()
	go s.manageSessions()
	go s.keepHeartbeat(ctx)

	return s.err
}

func (s *Server) serve() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleConnections())

	s.Server = &http.Server{
		Addr:    s.opts.address,
		Handler: mux,
	}
	log.Infof("[websocket] server listening on: %s", s.opts.lis.Addr().String())
	if s.err = s.Server.Serve(s.opts.lis); s.err != nil && !errors.Is(s.err, http.ErrServerClosed) {
		log.Errorf("server error: %v", s.err)
	}
}

func (s *Server) manageSessions() {
	for {
		select {
		case sess := <-s.register:
			s.sessionMgr.Add(sess)
			if s.OnOpenFunc != nil {
				s.OnOpenFunc(sess)
			}
		case sess := <-s.unregister:
			s.sessionMgr.Delete(sess)
			if s.OnCloseFunc != nil {
				s.OnCloseFunc(sess)
			}
		}
	}
}

func (s *Server) handleConnections() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 连接数限制
		if cnt := s.sessionMgr.Len(); cnt >= s.opts.maxConnections {
			w.WriteHeader(http.StatusServiceUnavailable)
			log.Warnf("StatusServiceUnavailable. over maxConnections(%d)", cnt)
			return
		}

		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Errorf("upgrade error: %v", err)
			return
		}

		sess := NewSession(s, conn)
		sess.listen()
		s.register <- sess
	}
}

// server检查所有session心跳
func (s *Server) keepHeartbeat(ctx context.Context) {
	defer func() {
		if err := s.recoveryServer(); err != nil {
			log.Error("keepHeartbeat %v", err)
		}
	}()

	ticker := time.NewTicker(s.opts.heartInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-1 * s.opts.heartDeadline)
			threshold := time.Now().Add(-1 * s.opts.heartThreshold)
			s.sessionMgr.Range(func(sess *Session) {
				if sess.LastActive().Before(cutoff) {
					log.Warnf("key %s heartbeat dead line.", sess.id)
					sess.Close()
				} else if sess.LastActive().Before(threshold) {
					log.Warnf("key %s heartbeat threshold. send ping", sess.id)
					_ = sess.Send(mustMarshal(&proto.Payload{Type: int32(proto.Ping)}))
				}
			})
		}
	}
}

// Stop stop the Websocket server.
func (s *Server) Stop(ctx context.Context) error {
	// 1. 停止HTTP服务器
	if err := s.Shutdown(ctx); err != nil {
		return err
	}

	// 2. 关闭所有会话
	var wg sync.WaitGroup
	s.sessionMgr.Range(func(sess *Session) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sess.Close()
		}()
	})

	// 3. 等待会话关闭完成
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	// 4. 关闭任务循环
	s.loop.Stop()

	log.Info("[webSocket] server stopping")
	return nil
}

// Dispatch 消息分发
func (s *Server) dispatchMessage(sess *Session, data []byte) error {
	var err error
	var p proto.Payload
	if err = gproto.Unmarshal(data, &p); err != nil {
		return err
	}
	ctx := context.WithValue(context.Background(), "session", sess)

	switch p.Type {
	case int32(proto.Ping):
		p.Op = 1
		p.Type = int32(proto.Pong)
		p.Body = nil
		err = sess.Send(mustMarshal(&p)) //sess.sendPayload(&p)

	case int32(proto.Push):
		err = sess.Send(mustMarshal(&p))

	case int32(proto.Request):
		p.Type = int32(proto.Response)
		err = s.operate(ctx, sess, &p)

	default:
		log.Warnf("Unkonwn Payload type(%+v). key %s body=%+v", p.Type, sess.id, string(p.Body))
	}

	return err
}

/*
	Client          Server
	|                |
	|--- cmd:1001 -->|  // LoginReq
	|                |
	|<-- cmd:1002 ---|  // LoginResp（自动由 reqCmd=1001 映射）
*/
// Operate 执行操作 {type:Request seq:1 body{ops:1001 Data:dataReq}} ->  {type:Push seq:0 body{ops:1002 Data:dataRsp}} + {type:Respond seq:1 body{ops:1001 Data:dataRsp}}
func (s *Server) operate(ctx context.Context, sess *Session, p *proto.Payload) (err error) {
	if p.Type == int32(proto.Request) {
		p.Type = int32(proto.Response)
	}

	reqBody := &proto.Body{}
	if err = gproto.Unmarshal(p.Body, reqBody); err != nil {
		return
	}
	srv := s.m
	md, ok := srv.md[reqBody.Ops]
	if !ok {
		log.Warnf("Unkonwn Ops(%+v) ", reqBody.Ops)
		// 返回错误响应给客户端
		p.Code = 505 // 或自定义错误码
		return sess.Send(mustMarshal(p))
	}
	reply, errCode := md.Handler(srv.server, ctx, reqBody.Data, s.interceptor)
	if errCode != nil {
		log.Errorf("errCord=%+v p:%+v", errCode, p)
	}
	p.Code = 0
	p.Body = reply

	// send. 将回调handle的结果send给client
	err = sess.Send(mustMarshal(p))
	return
}

func (s *Server) interceptor(ctx context.Context, req interface{}, args *UnaryServerInfo, handler UnaryHandler) ([]byte, error) {
	var (
		i     int
		chain UnaryHandler
	)
	n := len(s.handlers)
	if n == 0 {
		return handler(ctx, req)
	}
	chain = func(ic context.Context, ir interface{}) ([]byte, error) {
		if i == n-1 {
			return handler(ic, ir)
		}
		i++
		return s.handlers[i](ic, ir, args, chain)
	}
	return s.handlers[0](ctx, req, args, chain)
}

func (s *Server) recoveryServer() (err error) {
	if rerr := recover(); rerr != nil {
		const size = 64 << 10
		buf := make([]byte, size)
		rs := runtime.Stack(buf, false)
		if rs > size {
			rs = size
		}
		buf = buf[:rs]
		pl := fmt.Sprintf("panic: %v\n%s\n", rerr, buf)
		fmt.Fprintf(os.Stderr, pl)
		err = fmt.Errorf(pl)
	}
	return
}

// recovery is a server interceptor that recovers from any panics.
func (s *Server) recovery() UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) ([]byte, error) {
		var (
			err  error
			resp []byte
		)
		defer func() {
			if rerr := recover(); rerr != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				rs := runtime.Stack(buf, false)
				if rs > size {
					rs = size
				}
				buf = buf[:rs]
				pl := fmt.Sprintf("websocket server panic: %v\n%v\n%s\n", req, rerr, buf)
				fmt.Fprintf(os.Stderr, pl)
				log.Error(pl)
				err = status.Errorf(codes.Unknown, "server err")
			}
		}()
		resp, err = handler(ctx, req)
		return resp, err
	}
}
