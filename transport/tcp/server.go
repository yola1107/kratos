package tcp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/yola1107/kratos/v2/internal/endpoint"
	"github.com/yola1107/kratos/v2/internal/host"
	"github.com/yola1107/kratos/v2/internal/matcher"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware"
	"github.com/yola1107/kratos/v2/transport"
	"github.com/yola1107/kratos/v2/transport/tcp/internal/bucket"
	"github.com/yola1107/kratos/v2/transport/tcp/internal/round"
	xtime "github.com/yola1107/kratos/v2/transport/tcp/internal/time"
	"github.com/yola1107/kratos/v2/transport/tcp/proto"
	"github.com/zhenjl/cityhash"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	gproto "google.golang.org/protobuf/proto"
)

var (
	_ transport.Server     = (*Server)(nil)
	_ transport.Endpointer = (*Server)(nil)
)

// ServerConfig defines the configuration for the TCP server
type ServerConfig struct {
	TCP       *TCP
	Websocket *Websocket
	Protocol  *Protocol
	Auth      *Auth
	Bucket    *Bucket
	ChanSize  *ChanSize
}

type TCP struct {
	// Bind         []string
	Sndbuf       int
	Rcvbuf       int
	KeepAlive    bool
	Reader       int
	ReadBuf      int
	ReadBufSize  int
	Writer       int
	WriteBuf     int
	WriteBufSize int
}

// Websocket is websocket config.
type Websocket struct {
	Bind        []string
	TLSOpen     bool
	TLSBind     []string
	CertFile    string
	PrivateFile string
}

type Protocol struct {
	Proxy            bool
	Timer            int
	TimerSize        int
	SvrProto         int
	CliProto         int
	HandshakeTimeout xtime.Duration
	HeartbeatTimeout xtime.Duration
}

type Auth struct {
	Open bool
}

type Bucket struct {
	Size    int
	Channel int
}

type ChanSize struct {
	Push       int
	Close      int
	Disconnect int
}

// PushData represents data to be pushed to a client
type PushData struct {
	Mid  string
	Ops  int32
	Data []byte
}

// ChanList holds the communication channels for the server
type ChanList struct {
	PushChan       chan *PushData
	CloseChan      chan string
	DisconnectChan chan string
}

// MethodDesc describes an RPC method
type MethodDesc struct {
	Ops        int32
	MethodName string
	Handler    methodHandler
}

type methodHandler func(srv interface{}, ctx context.Context, data []byte, interceptor UnaryServerInterceptor) ([]byte, error)

// ServiceDesc describes a service and its methods
type ServiceDesc struct {
	ServiceName string
	// The pointer to the service interface. Used to check whether the user
	// provided implementation satisfies the interface requirements.
	HandlerType interface{}
	Methods     []MethodDesc
}

type service struct {
	server interface{}
	md     map[int32]*MethodDesc
}

// ServerOption is TCP server option.
type ServerOption func(o *Server)

// Network with server network.
func Network(network string) ServerOption {
	return func(s *Server) {
		s.network = network
	}
}

// Address with server address.
func Address(addr string) ServerOption {
	return func(s *Server) {
		s.address = addr
	}
}

// Endpoint with server address.
func Endpoint(endpoint *url.URL) ServerOption {
	return func(s *Server) {
		s.endpoint = endpoint
	}
}

// Timeout with server timeout.
func Timeout(timeout time.Duration) ServerOption {
	return func(s *Server) {
		s.timeout = timeout
	}
}

// Middleware with server middleware.
func Middleware(m ...middleware.Middleware) ServerOption {
	return func(o *Server) {
		o.middleware.Use(m...)
	}
}

// Server is an TCP server wrapper.
type Server struct {
	network    string
	address    string
	tlsConf    *tls.Config
	lis        net.Listener
	err        error
	endpoint   *url.URL
	timeout    time.Duration
	middleware matcher.Matcher

	c              *ServerConfig
	round          *round.Round
	buckets        []*bucket.Bucket
	bucketIdx      uint32
	serverID       string
	unaryInts      []UnaryServerInterceptor
	m              *service
	pushChan       chan *PushData
	closeChan      chan string
	disconnectChan chan string
}

// NewServer creates an TCP server by options.
func NewServer(opts ...ServerOption) *Server {
	s := &Server{
		network:    "tcp",
		address:    ":3101",
		timeout:    1 * time.Second,
		middleware: matcher.New(),
		c: &ServerConfig{
			TCP: &TCP{
				Sndbuf:       4096,
				Rcvbuf:       4096,
				KeepAlive:    false,
				Reader:       32,
				ReadBuf:      1024,
				ReadBufSize:  8192,
				Writer:       32,
				WriteBuf:     1024,
				WriteBufSize: 8192,
			},
			Websocket: &Websocket{
				Bind: []string{":3102"},
			},
			Protocol: &Protocol{
				Proxy:            false,
				Timer:            32,
				TimerSize:        2048,
				CliProto:         5,
				SvrProto:         10,
				HandshakeTimeout: xtime.Duration(15 * time.Second),
				HeartbeatTimeout: xtime.Duration(6 * time.Second),
			},
			Auth: &Auth{
				Open: false,
			},
			Bucket: &Bucket{
				Size:    32,
				Channel: 1024,
			},
			ChanSize: &ChanSize{
				Push:       2048,
				Close:      1024,
				Disconnect: 1024,
			},
		},
	}
	for _, o := range opts {
		o(s)
	}

	// init round
	s.round = round.NewRound(round.RoundOptions{
		Reader:       s.c.TCP.Reader,
		ReadBuf:      s.c.TCP.ReadBuf,
		ReadBufSize:  s.c.TCP.ReadBufSize,
		Writer:       s.c.TCP.Writer,
		WriteBuf:     s.c.TCP.WriteBuf,
		WriteBufSize: s.c.TCP.WriteBufSize,
		Timer:        s.c.Protocol.Timer,
		TimerSize:    s.c.Protocol.TimerSize,
	})

	// init bucket
	s.buckets = make([]*bucket.Bucket, s.c.Bucket.Size)
	s.bucketIdx = uint32(s.c.Bucket.Size)
	for i := 0; i < s.c.Bucket.Size; i++ {
		s.buckets[i] = bucket.NewBucket(s.c.Bucket.Channel)
	}

	// init chan
	s.pushChan = make(chan *PushData, s.c.ChanSize.Push)
	s.closeChan = make(chan string, s.c.ChanSize.Close)
	s.disconnectChan = make(chan string, s.c.ChanSize.Disconnect)

	s.Use(s.unaryServerInterceptor())
	return s
}

// Start starts the TCP server
func (s *Server) Start(ctx context.Context) error {
	log.Infof("[TCP] server listening on: %s", s.lis.Addr().String())
	if err := s.listenAndEndpoint(); err != nil {
		return err
	}
	err := s.StartTCP(runtime.NumCPU())
	return err
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	log.Infof("[TCP] server stopping")
	return nil
}

// GetBucket get the bucket by subkey.
func (s *Server) GetBucket(subKey string) *bucket.Bucket {
	idx := cityhash.CityHash32([]byte(subKey), uint32(len(subKey))) % s.bucketIdx
	return s.buckets[idx]
}

// Use adds interceptors to the server
func (s *Server) Use(handlers ...UnaryServerInterceptor) *Server {
	if len(s.unaryInts)+len(handlers) > math.MaxInt8/2 {
		panic("tcp: too many interceptors")
	}
	s.unaryInts = append(s.unaryInts, handlers...)
	return s
}

// Endpoint returns the server endpoint
func (s *Server) Endpoint() (*url.URL, error) {
	if err := s.listenAndEndpoint(); err != nil {
		return nil, err
	}
	return s.endpoint, nil
}

// listenAndEndpoint sets up the listener and endpoint
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
		s.endpoint = endpoint.NewEndpoint(endpoint.Scheme("tcp", s.tlsConf != nil), addr)
	}
	return s.err
}

// RegisterService registers a service with the server
func (s *Server) RegisterService(sd *ServiceDesc, ss interface{}) *ChanList {
	ht := reflect.TypeOf(sd.HandlerType).Elem()
	st := reflect.TypeOf(ss)
	if !st.Implements(ht) {
		log.Fatalf("tcp: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
	}
	if s.m != nil {
		log.Fatalf("tcp: Server.RegisterService found duplicate service registration for %q", sd.ServiceName)
	}
	srv := &service{
		server: ss,
		md:     make(map[int32]*MethodDesc),
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		srv.md[d.Ops] = d
	}
	s.m = srv
	cl := &ChanList{
		PushChan:       s.pushChan,
		CloseChan:      s.closeChan,
		DisconnectChan: s.disconnectChan,
	}
	// close
	go func() {
		for {
			mid := <-s.closeChan
			if channel := s.GetBucket(mid).Channel(mid); channel != nil {
				channel.Close()
			}
		}
	}()
	// push
	go func() {
		for {
			pd := <-s.pushChan
			s.PushByChannel(pd.Mid, pd.Ops, pd.Data)
		}
	}()
	return cl
}

// PushByChannel pushes data to a specific channel
func (s *Server) PushByChannel(sessionID string, ops int32, data []byte) {
	ch := s.GetBucket(sessionID).Channel(sessionID)
	if ch == nil {
		log.Warnf("channel not found for session %s", sessionID)
		return
	}

	body, err := gproto.Marshal(&proto.Body{Ops: ops, Data: data})
	if err != nil {
		log.Errorf("marshal push body error: %v", err)
		return
	}

	payload := &proto.Payload{
		Place: proto.PlaceServer,
		Type:  int32(proto.Push),
		Body:  body,
	}

	if err := ch.Push(payload); err != nil {
		log.Errorf("push failed for channel %s: %v", sessionID, err)
	}
}

// Operate processes an incoming payload
func (s *Server) Operate(ctx context.Context, p *proto.Payload) error {
	p.Place = proto.PlaceServer

	switch p.Type {
	case int32(proto.Ping):
		p.Type = int32(proto.Pong)
		p.Body = nil
		return nil

	case int32(proto.Request):
		p.Type = int32(proto.Response)
		reqBody := &proto.Body{}
		if err := gproto.Unmarshal(p.Body, reqBody); err != nil {
			return status.Errorf(codes.InvalidArgument, "failed to unmarshal request body: %v", err)
		}
		md, ok := s.m.md[reqBody.Ops]
		if !ok {
			return status.Errorf(codes.Unimplemented, "Unimplemented Ops=%d.", reqBody.Ops)
		}
		reply, errCode := md.Handler(s.m.server, ctx, reqBody.Data, s.interceptor)
		st, _ := status.FromError(errCode)
		p.Code = int32(st.Code())
		body := &proto.Body{
			Ops:  reqBody.Ops,
			Data: reply,
		}
		bodyData, _ := gproto.Marshal(body)
		p.Body = bodyData
		return errCode

	default:
		log.Warnf("unknown payload.Type: %v", p.Type)
		return nil
	}
}

// interceptor chain
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

// unified interceptor entry with middleware
func (s *Server) unaryServerInterceptor() UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *UnaryServerInfo, handler UnaryHandler) ([]byte, error) {
		if s.timeout > 0 {
			var cancel context.CancelFunc
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
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || strings.HasPrefix(err.Error(), "panic:") {
				st, _ := status.FromError(err)
				log.Errorf("[TCP] unary method=[%s] unexpected err. st.Code=%d(%v) st.Message=%v", info.FullMethod, st.Code(), st.Code(), err)
			}
			return nil, err
		}

		data, ok := reply.([]byte)
		if !ok {
			return nil, fmt.Errorf("[TCP] unary method=[%s] must return []byte, got %T", info.FullMethod, reply)
		}
		return data, nil
	}
}
