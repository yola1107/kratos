package gnet

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/panjf2000/gnet/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	gproto "google.golang.org/protobuf/proto"

	ic "github.com/yola1107/kratos/v2/internal/context"
	"github.com/yola1107/kratos/v2/internal/endpoint"
	"github.com/yola1107/kratos/v2/internal/host"
	"github.com/yola1107/kratos/v2/internal/matcher"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware"
	"github.com/yola1107/kratos/v2/transport"
	tcpproto "github.com/yola1107/kratos/v2/transport/tcp/proto"
)

var (
	_ transport.Server     = (*Server)(nil)
	_ transport.Endpointer = (*Server)(nil)
	_ gnet.EventHandler    = (*Server)(nil)
)

// ServerOption configures the gnet server.
type ServerOption func(*Server)

// Network sets the server network scheme (tcp, tcp4, tcp6, udp...).
func Network(network string) ServerOption {
	return func(s *Server) {
		s.network = network
	}
}

// Address sets the server listen address (host:port).
func Address(addr string) ServerOption {
	return func(s *Server) {
		s.address = addr
	}
}

// Endpoint sets the registry endpoint.
func Endpoint(endpoint *url.URL) ServerOption {
	return func(s *Server) {
		s.endpoint = endpoint
	}
}

// Timeout sets handler timeout.
func Timeout(timeout time.Duration) ServerOption {
	return func(s *Server) {
		s.timeout = timeout
	}
}

// Logger deprecated: retained for backward compatibility.
func Logger(log.Logger) ServerOption {
	return func(*Server) {}
}

// Middleware adds middleware for selector matching.
func Middleware(m ...middleware.Middleware) ServerOption {
	return func(s *Server) {
		s.middleware.Use(m...)
	}
}

// Options forwards raw gnet options.
func Options(opts ...gnet.Option) ServerOption {
	return func(s *Server) {
		s.opts = append(s.opts, opts...)
	}
}

// Server is a gnet-based transport server.
type Server struct {
	gnet.BuiltinEventEngine

	baseCtx    context.Context
	protoAddr  string
	network    string
	address    string
	endpoint   *url.URL
	err        error
	timeout    time.Duration
	middleware matcher.Matcher
	opts       []gnet.Option

	srv *service
}

// NewServer creates a gnet server with options.
func NewServer(opts ...ServerOption) *Server {
	s := &Server{
		baseCtx:    context.Background(),
		network:    "tcp",
		address:    ":3200",
		timeout:    time.Second,
		middleware: matcher.New(),
		srv:        &service{md: make(map[int32]*MethodDesc)},
	}
	for _, o := range opts {
		o(s)
	}
	s.protoAddr = s.buildProtoAddr()
	return s
}

// Use uses a service middleware with selector.
// selector examples:
//   - '/*'
//   - '/helloworld.v1.Greeter/*'
//   - '/helloworld.v1.Greeter/SayHello'
func (s *Server) Use(selector string, m ...middleware.Middleware) {
	s.middleware.Add(selector, m...)
}

// RegisterService registers a service with the gnet server.
func (s *Server) RegisterService(sd *ServiceDesc, srv interface{}) {
	s.srv.register(sd, srv)
}

// Endpoint returns a real address to registry endpoint.
func (s *Server) Endpoint() (*url.URL, error) {
	if err := s.listenAndEndpoint(); err != nil {
		return nil, err
	}
	return s.endpoint, nil
}

// Start starts the gnet server.
func (s *Server) Start(ctx context.Context) error {
	if _, err := s.Endpoint(); err != nil {
		return err
	}
	s.baseCtx = ctx
	log.Infof("[gnet] server listening on: %s", s.protoAddr)
	return gnet.Run(s, s.protoAddr, s.opts...)
}

// Stop stops the gnet server.
func (s *Server) Stop(ctx context.Context) error {
	log.Info("[gnet] server stopping")
	return gnet.Stop(ctx, s.protoAddr)
}

// OnTraffic is triggered when data is available.
func (s *Server) OnTraffic(c gnet.Conn) (action gnet.Action) {
	for {
		if c.InboundBuffered() < 4 {
			return gnet.None
		}
		header, err := c.Peek(4)
		if err != nil {
			log.Warnf("[gnet] peek header error: %v", err)
			return gnet.Close
		}
		size := int(binary.BigEndian.Uint32(header))
		if size <= 0 {
			log.Warnf("[gnet] invalid frame size=%d", size)
			return gnet.Close
		}
		if c.InboundBuffered() < 4+size {
			return gnet.None
		}
		if _, err = c.Discard(4); err != nil {
			log.Warnf("[gnet] discard header error: %v", err)
			return gnet.Close
		}
		frame, err := c.Peek(size)
		if err != nil {
			log.Warnf("[gnet] peek frame error: %v", err)
			return gnet.Close
		}
		p := &tcpproto.Payload{}
		if err := gproto.Unmarshal(frame, p); err != nil {
			log.Warnf("[gnet] unmarshal payload error: %v", err)
			_, _ = c.Discard(size)
			continue
		}
		if _, err = c.Discard(size); err != nil {
			log.Warnf("[gnet] discard frame error: %v", err)
			return gnet.Close
		}

		resp, err := s.handlePayload(c, p)
		if err != nil {
			log.Warnf("[gnet] handle payload error: %v", err)
		}
		if resp == nil {
			continue
		}
		out, err := encodePayload(resp)
		if err != nil {
			log.Warnf("[gnet] encode payload error: %v", err)
			continue
		}
		if _, err = c.Write(out); err != nil {
			log.Warnf("[gnet] write response error: %v", err)
			return gnet.Close
		}
	}
}

func (s *Server) handlePayload(_ gnet.Conn, p *tcpproto.Payload) (*tcpproto.Payload, error) {
	ctx, cancel := ic.Merge(context.Background(), s.baseCtx)
	defer cancel()

	tr := &Transport{
		reqHeader:  headerCarrier{},
		respHeader: headerCarrier{},
	}
	if s.endpoint != nil {
		tr.endpoint = s.endpoint.String()
	}
	ctx = transport.NewServerContext(ctx, tr)

	if s.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	switch tcpproto.Pattern(p.Type) {
	case tcpproto.Ping:
		p.Type = int32(tcpproto.Pong)
		p.Code = 0
		p.Body = nil
		return p, nil
	case tcpproto.Request:
		return s.operate(ctx, p)
	default:
		return nil, nil
	}
}

func (s *Server) operate(ctx context.Context, p *tcpproto.Payload) (*tcpproto.Payload, error) {
	reqBody := &tcpproto.Body{}
	if err := gproto.Unmarshal(p.Body, reqBody); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to unmarshal body: %v", err)
	}
	md, ok := s.srv.md[reqBody.Ops]
	if !ok {
		p.Type = int32(tcpproto.Response)
		p.Place = tcpproto.PlaceServer
		p.Code = int32(codes.Unimplemented)
		p.Body = nil
		return p, status.Errorf(codes.Unimplemented, "Unimplemented Ops=%d", reqBody.Ops)
	}
	fullMethod := fmt.Sprintf("/%s/%s", s.srv.name, md.MethodName)
	tr := transportContext(ctx)
	if tr != nil {
		tr.operation = fullMethod
	}

	reply, errCode := md.Handler(s.srv.server, ctx, reqBody.Data, s.unaryServerInterceptor())
	st, _ := status.FromError(errCode)

	respBody, marshalErr := gproto.Marshal(&tcpproto.Body{Ops: reqBody.Ops, Data: reply})
	if marshalErr != nil {
		return nil, marshalErr
	}

	p.Type = int32(tcpproto.Response)
	p.Place = tcpproto.PlaceServer
	p.Code = int32(st.Code())
	p.Body = respBody
	return p, errCode
}

func (s *Server) listenAndEndpoint() error {
	if s.endpoint == nil {
		addr, err := host.Extract(s.address, nil)
		if err != nil {
			s.err = err
			return err
		}
		s.endpoint = endpoint.NewEndpoint(endpoint.Scheme("gnet", false), addr)
	}
	return s.err
}

func (s *Server) buildProtoAddr() string {
	if strings.Contains(s.address, "://") {
		return s.address
	}
	if s.network == "" {
		s.network = "tcp"
	}
	return fmt.Sprintf("%s://%s", s.network, s.address)
}

func (s *Server) unaryServerInterceptor() UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *UnaryServerInfo, handler UnaryHandler) ([]byte, error) {
		h := func(ctx context.Context, req any) (any, error) {
			return handler(ctx, req)
		}
		if next := s.middleware.Match(info.FullMethod); len(next) > 0 {
			h = middleware.Chain(next...)(h)
		}

		if s.timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.timeout)
			defer cancel()
		}
		reply, err := h(ctx, req)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || strings.HasPrefix(err.Error(), "panic:") {
				st, _ := status.FromError(err)
				log.Errorf("[gnet] unary method=[%s] unexpected err. st.Code=%d(%v) st.Message=%v", info.FullMethod, st.Code(), st.Code(), err)
			}
			return nil, err
		}
		data, ok := reply.([]byte)
		if !ok {
			return nil, fmt.Errorf("[gnet] unary method=[%s] must return []byte, got %T", info.FullMethod, reply)
		}
		return data, nil
	}
}

func encodePayload(p *tcpproto.Payload) ([]byte, error) {
	body, err := gproto.Marshal(p)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(body)))
	copy(buf[4:], body)
	return buf, nil
}

func transportContext(ctx context.Context) *Transport {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return nil
	}
	gtr, ok := tr.(*Transport)
	if !ok {
		return nil
	}
	return gtr
}
