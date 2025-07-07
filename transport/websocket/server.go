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
	"strings"
	"time"

	gproto "github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	kerrors "github.com/yola1107/kratos/v2/errors"
	ic "github.com/yola1107/kratos/v2/internal/context"
	"github.com/yola1107/kratos/v2/internal/endpoint"
	"github.com/yola1107/kratos/v2/internal/host"
	"github.com/yola1107/kratos/v2/internal/matcher"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware"
	"github.com/yola1107/kratos/v2/transport"
	"github.com/yola1107/kratos/v2/transport/websocket/proto"
	"google.golang.org/grpc/codes"
)

var (
	_ transport.Server     = (*Server)(nil)
	_ transport.Endpointer = (*Server)(nil)
)

const (
	CtxSessionKey    = "session"
	CtxSessionIDKey  = "sessionID"
	CtxSessionUIDKey = "userID"
)

// ServerOption is a Websocket server option.
type ServerOption func(*Server)

func Network(network string) ServerOption {
	return func(o *Server) { o.network = network }
}
func Address(addr string) ServerOption {
	return func(o *Server) { o.address = addr }
}
func Path(path string) ServerOption {
	return func(o *Server) { o.path = path }
}
func Endpoint(u *url.URL) ServerOption {
	return func(o *Server) { o.endpoint = u }
}
func TlsConf(tlsConfig *tls.Config) ServerOption {
	return func(o *Server) { o.tlsConf = tlsConfig }
}
func MaxConnLimit(maxConnLimit int32) ServerOption {
	return func(o *Server) { o.maxConnLimit = maxConnLimit }
}
func Timeout(d time.Duration) ServerOption {
	return func(o *Server) { o.timeout = d }
}
func SessionConf(c *SessionConfig) ServerOption {
	return func(o *Server) { o.sessionConf = c }
}
func Heartbeat(d, i, w time.Duration) ServerOption {
	return func(o *Server) {
		o.sessionConf.ReadDeadline, o.sessionConf.PingInterval, o.sessionConf.WriteTimeout = d, i, w
	}
}
func SentChanSize(size int) ServerOption {
	return func(o *Server) { o.sessionConf.SendChanSize = size }
}
func Middleware(m ...middleware.Middleware) ServerOption {
	return func(o *Server) { o.middleware.Use(m...) }
}

// Server is a Websocket server wrapper.
type Server struct {
	*http.Server
	baseCtx      context.Context
	lis          net.Listener
	tlsConf      *tls.Config
	endpoint     *url.URL
	err          error
	path         string
	network      string
	address      string
	timeout      time.Duration
	maxConnLimit int32
	sessionConf  *SessionConfig
	middleware   matcher.Matcher          // 中间件
	upgrader     *websocket.Upgrader      // WebSocket升级器
	sessionMgr   *SessionManager          // 会话管理
	unaryInts    []UnaryServerInterceptor // 拦截器链
	m            *service                 // 注册的服务
}

// NewServer creates a Websocket server by options.
func NewServer(opts ...ServerOption) *Server {
	srv := &Server{
		baseCtx:    context.Background(),
		network:    "tcp",
		address:    ":0",
		path:       "/",
		timeout:    5 * time.Second,
		middleware: matcher.New(),
		sessionConf: &SessionConfig{
			WriteTimeout: 10 * time.Second,
			PingInterval: 15 * time.Second,
			ReadDeadline: 60 * time.Second,
			SendChanSize: 128,
		},
		maxConnLimit: 10000,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		sessionMgr: NewSessionManager(),
	}
	for _, o := range opts {
		o(srv)
	}
	srv.Server = &http.Server{
		Addr:      srv.address,
		TLSConfig: srv.tlsConf,
	}
	// 使用CORS中间件包装处理函数
	http.Handle(srv.path, CORS(srv.handleConnections()))
	srv.Use(srv.unaryServerInterceptor())
	return srv
}

func (s *Server) Use(handlers ...UnaryServerInterceptor) *Server {
	if len(s.unaryInts)+len(handlers) > math.MaxInt8/2 {
		panic("websocket: server use too many handlers")
	}
	s.unaryInts = append(s.unaryInts, handlers...)
	return s
}

func (s *Server) Endpoint() (*url.URL, error) {
	if err := s.listenAndEndpoint(); err != nil {
		return nil, err
	}
	return s.endpoint, nil
}

func (s *Server) listenAndEndpoint() error {
	if s.lis == nil {
		lis, err := net.Listen(s.network, s.address)
		if err != nil {
			s.err = err
			return err
		}
		s.lis = lis
	}
	if s.endpoint == nil {
		addr, err := host.Extract(s.address, s.lis)
		if err != nil {
			s.err = err
			return err
		}
		s.endpoint = endpoint.NewEndpoint(endpoint.Scheme("ws", s.tlsConf != nil), addr)
	}
	return s.err
}

// Start start the Websocket server.
func (s *Server) Start(ctx context.Context) error {
	if err := s.listenAndEndpoint(); err != nil {
		return err
	}
	s.baseCtx = ctx
	s.BaseContext = func(net.Listener) context.Context {
		return ctx
	}
	log.Infof("[websocket] server listening on: %s", s.lis.Addr().String())
	var err error
	if s.tlsConf != nil {
		err = s.ServeTLS(s.lis, "", "")
	} else {
		err = s.Serve(s.lis)
	}
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) handleConnections() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cnt := s.sessionMgr.Len(); cnt >= s.maxConnLimit {
			w.WriteHeader(http.StatusServiceUnavailable)
			log.Warnf("[websocket] StatusServiceUnavailable. over maxConnections(%d)", cnt)
			return
		}

		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Errorf("[websocket] upgrade error: %v", err)
			return
		}

		_ = NewSession(s, conn, s.sessionConf)
	}
}

// Stop stop the Websocket server.
func (s *Server) Stop(ctx context.Context) error {
	log.Info("[webSocket] server stopping")

	// 停止HTTP服务器
	err := s.Shutdown(ctx)

	// 关闭所有会话
	s.sessionMgr.CloseAllSessions()

	return err
}

func (s *Server) OnSessionOpen(sess *Session) {
	s.sessionMgr.Add(sess)
	if s.m.connectFunc != nil {
		s.m.connectFunc(sess)
	}
}

func (s *Server) OnSessionClose(sess *Session) {
	if s.m.disconnectFunc != nil {
		s.m.disconnectFunc(sess)
	}
	s.sessionMgr.Delete(sess)
}

// DispatchMessage 消息分发
func (s *Server) DispatchMessage(sess *Session, data []byte) error {
	var err error
	var p proto.Payload
	if err = gproto.Unmarshal(data, &p); err != nil {
		return err
	}

	// 通过context传递给session等数据给调用方
	ctx := context.WithValue(s.baseCtx, CtxSessionKey, sess)
	ctx = context.WithValue(ctx, CtxSessionIDKey, sess.id)
	ctx = context.WithValue(ctx, CtxSessionUIDKey, sess.UID())

	switch p.Op {
	case proto.OpPing:
		p.Op = proto.OpPong
		p.Body = nil
		err = sess.SendPayload(&p)

	case proto.OpPong:
		// 回pong包. 不处理

	case proto.OpPush:
		// 收到push. 不处理

	case proto.OpRequest:
		err = s.operate(ctx, sess, &p)

	default:
		log.Warnf("[websocket] Unkonwn Payload op(%+v). key=%s ", p.Op, sess.id)
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
// Operate 执行操作 {type:Request seq:1 ops:1001 body{dataReq}} ->  {type:Push seq:0 ops:1002 body{dataRsp}} + {type:Respond seq:1 ops:1001 body{dataRsp}}
func (s *Server) operate(ctx context.Context, sess *Session, p *proto.Payload) (err error) {
	p.Op = proto.OpResponse
	p.Place = proto.PlaceServer

	srv := s.m
	md, ok := srv.md[p.Command]
	if !ok {
		p.Code = int32(codes.Unimplemented) // 	Unimplemented Code = 12 或自定义错误码503
		log.Warnf("[websocket] Unimplemented Command=%+v code=%d", p.Command, p.Code)
		return sess.SendPayload(p)
	}

	reply, errCode := md.Handler(srv.server, ctx, p.Body, s.interceptor)
	if errCode != nil {
		e := kerrors.FromError(errCode)
		p.Code = e.Code
		p.Body = nil
		err = e
	} else {
		p.Code = 0
		p.Body = reply
	}

	// send. 将回调handle的结果send给client
	return errors.Join(err, sess.SendPayload(p))
}

// 迭代方式执行拦截器链
func (s *Server) interceptor(ctx context.Context, req interface{}, args *UnaryServerInfo, handler UnaryHandler) ([]byte, error) {
	chain := handler
	// 反向包装中间件
	for i := len(s.unaryInts) - 1; i >= 0; i-- {
		chain = wrap(s.unaryInts[i], chain, args)
	}
	return chain(ctx, req)
}

func wrap(h UnaryServerInterceptor, next UnaryHandler, args *UnaryServerInfo) UnaryHandler {
	return func(ctx context.Context, req interface{}) ([]byte, error) {
		return h(ctx, req, args, next)
	}
}

func (s *Server) unaryServerInterceptor() UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *UnaryServerInfo, handler UnaryHandler) ([]byte, error) {
		ctx, cancel := ic.Merge(ctx, s.baseCtx)
		defer cancel()
		if s.timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, s.timeout)
			defer cancel()
		}
		h := func(ctx context.Context, req any) (any, error) {
			return handler(ctx, req)
		}
		if next := s.middleware.Match(info.FullMethod); len(next) > 0 {
			h = middleware.Chain(next...)(h)
		}
		reply, err := h(ctx, req)
		if err != nil {
			if strings.HasPrefix(err.Error(), "panic:") ||
				errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				e := kerrors.FromError(err) // st, _ := status.FromError(err)
				log.Errorf("[websocket] [%s] unexpected err. st.Code=%d st.Message=%v", info.FullMethod, e.Code, e.Message)
			}
			return nil, err
		}
		data, ok := reply.([]byte)
		if !ok {
			return nil, fmt.Errorf("[websocket] [%s] must return []byte, got %T", info.FullMethod, reply)
		}
		return data, nil
	}
}

// 示例：日志拦截器
func (s *Server) loggingInterceptor() UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) ([]byte, error) {
		log.Info("<logging>请求开始:", info.FullMethod)
		resp, err := handler(ctx, req)
		log.Info("<logging>请求结束:", info.FullMethod)
		return resp, err
	}
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 添加 CORS 相关头部
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Length, X-CSRF-Token, Token, session")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
