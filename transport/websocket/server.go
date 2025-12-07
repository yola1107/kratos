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

	kerrors "github.com/yola1107/kratos/v2/errors"
	ic "github.com/yola1107/kratos/v2/internal/context"
	"github.com/yola1107/kratos/v2/internal/endpoint"
	"github.com/yola1107/kratos/v2/internal/host"
	"github.com/yola1107/kratos/v2/internal/matcher"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware"
	"github.com/yola1107/kratos/v2/transport"
	"github.com/yola1107/kratos/v2/transport/websocket/proto"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc/codes"
	gproto "google.golang.org/protobuf/proto"
)

var (
	_ transport.Server     = (*Server)(nil)
	_ transport.Endpointer = (*Server)(nil)
)

const (
	CtxSessionKey   = "session"
	CtxSessionIDKey = "sessionID"

	DefaultMaxConnLimit = 10000
	DefaultTimeout      = 5 * time.Second
	DefaultReadBufSize  = 4096
	DefaultWriteBufSize = 4096
	DefaultSendChanSize = 32
	DefaultWriteTimeout = 10 * time.Second
	DefaultPingInterval = 15 * time.Second
	DefaultReadDeadline = 60 * time.Second
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
		timeout:    DefaultTimeout,
		middleware: matcher.New(),
		sessionConf: &SessionConfig{
			WriteTimeout: DefaultWriteTimeout,
			PingInterval: DefaultPingInterval,
			ReadDeadline: DefaultReadDeadline,
			SendChanSize: DefaultSendChanSize,
		},
		maxConnLimit: DefaultMaxConnLimit,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  DefaultReadBufSize,
			WriteBufferSize: DefaultWriteBufSize,
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

	http.Handle(srv.path, corsHandler(srv.handleConnections()))
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

// Start starts the websocket server
func (s *Server) Start(ctx context.Context) error {
	if err := s.listenAndEndpoint(); err != nil {
		return err
	}

	s.baseCtx = ctx
	s.BaseContext = func(net.Listener) context.Context { return ctx }

	log.Infof("[websocket] server listening on: %s", s.lis.Addr().String())

	if s.tlsConf != nil {
		return s.ServeTLS(s.lis, "", "")
	}
	return s.Serve(s.lis)
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

// Stop stops the websocket server gracefully
func (s *Server) Stop(ctx context.Context) error {
	log.Info("[websocket] server stopping")

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Shutdown HTTP server first
	if err := s.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Errorf("[websocket] server shutdown error: %v", err)
		return err
	}

	// Close listener and sessions
	if s.lis != nil {
		s.lis.Close()
	}
	s.sessionMgr.CloseAllSessions()

	log.Info("[websocket] server stopped gracefully")
	return nil
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

// DispatchMessage handles incoming messages
func (s *Server) DispatchMessage(sess *Session, data []byte) error {
	var p proto.Payload
	if err := gproto.Unmarshal(data, &p); err != nil {
		return err
	}

	ctx := context.WithValue(s.baseCtx, CtxSessionKey, sess)
	ctx = context.WithValue(ctx, CtxSessionIDKey, sess.id)

	switch p.Op {
	case proto.OpPing:
		return sess.SendPayload(&proto.Payload{Op: proto.OpPong})
	case proto.OpRequest:
		return s.operate(ctx, sess, &p)
	}
	return nil
}

// operate processes operation requests
func (s *Server) operate(ctx context.Context, sess *Session, p *proto.Payload) error {
	p.Op, p.Place = proto.OpResponse, proto.PlaceServer

	srv := s.m
	md, ok := srv.md[p.Command]
	if !ok {
		p.Code = int32(codes.Unimplemented)
		log.Warnf("[websocket] unimplemented command=%d, session=%s", p.Command, sess.ID())
		return sess.SendPayload(p)
	}

	reply, err := md.Handler(srv.server, ctx, p.Body, s.interceptor)
	if err != nil {
		e := kerrors.FromError(err)
		p.Code, p.Body = e.Code, nil
		log.Errorf("[websocket] handler error command=%d, session=%s: %v", p.Command, sess.ID(), e.Message)
	} else {
		p.Code, p.Body = 0, reply
	}

	return sess.SendPayload(p)
}

// interceptor executes interceptor chain
func (s *Server) interceptor(ctx context.Context, req interface{}, args *UnaryServerInfo, handler UnaryHandler) ([]byte, error) {
	chain := handler
	for i := len(s.unaryInts) - 1; i >= 0; i-- {
		chain = func(h UnaryServerInterceptor, next UnaryHandler) UnaryHandler {
			return func(ctx context.Context, req interface{}) ([]byte, error) {
				return h(ctx, req, args, next)
			}
		}(s.unaryInts[i], chain)
	}
	return chain(ctx, req)
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
				e := kerrors.FromError(err)
				log.Errorf("[websocket] [%s] unexpected error: code=%d message=%v", info.FullMethod, e.Code, e.Message)
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

func corsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
