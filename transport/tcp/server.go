package tcp

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"time"

	gproto "github.com/golang/protobuf/proto"
	"github.com/zhenjl/cityhash"

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
)

var (
	_ transport.Server     = (*Server)(nil)
	_ transport.Endpointer = (*Server)(nil)
)

var (
	_defaultSerConf = &ServerConfig{
		TCP: &TCP{
			//Bind:         []string{":3101"},
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
		//TCP: &TCP{
		//	Bind: []string{":3101"},
		//},
		Websocket: &Websocket{
			Bind: []string{":3102"},
		},
		Protocol: &Protocol{
			Proxy:            false,
			Timer:            32,
			TimerSize:        2048,
			CliProto:         5, //5
			SvrProto:         10,
			HandshakeTimeout: xtime.Duration(time.Second * 15),
			HeartbeatTimeout: xtime.Duration(time.Second * 6),
		},
		//Auth: &Auth{
		//	Open:  false,
		//	AppID: "auth",
		//},
		Bucket: &Bucket{
			Size:    32,
			Channel: 1024,
		},
		ChanSize: &ChanSize{
			Push:       2048,
			Close:      1024,
			Disconnect: 1024,
		},
	}
	_abortIndex int8 = math.MaxInt8 / 2
)

// Config is comet config.
type (
	ServerConfig struct {
		TCP       *TCP
		Websocket *Websocket
		Protocol  *Protocol
		Bucket    *Bucket
		ChanSize  *ChanSize
	}

	// TCP is tcp config.
	TCP struct {
		//Bind         []string
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
	Websocket struct {
		Bind        []string
		TLSOpen     bool
		TLSBind     []string
		CertFile    string
		PrivateFile string
	}

	// Protocol is proto config.
	Protocol struct {
		Proxy            bool
		Timer            int
		TimerSize        int
		SvrProto         int
		CliProto         int
		HandshakeTimeout xtime.Duration
		HeartbeatTimeout xtime.Duration
	}

	//type Auth struct {
	//	Open  bool
	//	AppID string
	//}

	// Bucket is bucket config.
	Bucket struct {
		Size    int
		Channel int
	}

	// ChanSize
	ChanSize struct {
		Push       int
		Close      int
		Disconnect int
	}
)

// PushData
type PushData struct {
	Mid  string
	Ops  int32
	Data []byte
}

// ChanList
type ChanList struct {
	PushChan       chan *PushData
	CloseChan      chan string
	DisconnectChan chan string
}

type methodHandler func(srv interface{}, ctx context.Context, data []byte, interceptor UnaryServerInterceptor) ([]byte, error)

type MethodDesc struct {
	Ops        int32
	MethodName string
	Handler    methodHandler
}

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

//----------------------------------------------------------------------------------------------

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
	return func(s *Server) {
		s.middleware.Use(m...)
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
	round          *round.Round     // accept round store
	buckets        []*bucket.Bucket // subkey bucket
	bucketIdx      uint32
	serverID       string
	handlers       []UnaryServerInterceptor
	m              *service
	pushChan       chan *PushData
	closeChan      chan string
	disconnectChan chan string
}

// NewServer creates an TCP server by options.
func NewServer(opts ...ServerOption) *Server {
	conf := _defaultSerConf
	roundConfig := round.RoundOptions{
		Reader:       conf.TCP.Reader,
		ReadBuf:      conf.TCP.ReadBuf,
		ReadBufSize:  conf.TCP.ReadBufSize,
		Writer:       conf.TCP.Writer,
		WriteBuf:     conf.TCP.WriteBuf,
		WriteBufSize: conf.TCP.WriteBufSize,
		Timer:        conf.Protocol.Timer,
		TimerSize:    conf.Protocol.TimerSize,
	}
	s := &Server{
		network:    "tcp",
		address:    ":6000",
		timeout:    1 * time.Second,
		middleware: matcher.New(),

		c:     conf,
		round: round.NewRound(roundConfig),
	}

	// init bucket
	s.buckets = make([]*bucket.Bucket, conf.Bucket.Size)
	s.bucketIdx = uint32(conf.Bucket.Size)
	for i := 0; i < conf.Bucket.Size; i++ {
		s.buckets[i] = bucket.NewBucket(conf.Bucket.Channel)
	}
	s.pushChan = make(chan *PushData, conf.ChanSize.Push)
	s.closeChan = make(chan string, conf.ChanSize.Close)
	s.disconnectChan = make(chan string, conf.ChanSize.Disconnect)

	for _, o := range opts {
		o(s)
	}

	return s
}

func (s *Server) Start(context.Context) error {
	log.Infof("[TCP] server listening on: %s", s.lis.Addr().String())
	if err := s.listenAndEndpoint(); err != nil {
		return s.err
	}
	err := s.StartTCP(runtime.NumCPU())
	return err
}

func (s *Server) Stop(context.Context) error {
	log.Infof("[TCP] server stopping")
	return nil
}

// Bucket get the bucket by subkey.
func (s *Server) GetBucket(subKey string) *bucket.Bucket {
	idx := cityhash.CityHash32([]byte(subKey), uint32(len(subKey))) % s.bucketIdx
	return s.buckets[idx]
}

//// Use uses a service middleware with selector.
//// selector:
////   - '/*'
////   - '/helloworld.v1.Greeter/*'
////   - '/helloworld.v1.Greeter/SayHello'
//func (s *Server) Use(selector string, m ...middleware.Middleware) {
//	s.middleware.Add(selector, m...)
//}

func (s *Server) Use(handlers ...UnaryServerInterceptor) *Server {
	finalSize := len(s.handlers) + len(handlers)
	if finalSize >= int(_abortIndex) {
		panic("comet: server use too many handlers")
	}
	mergedHandlers := make([]UnaryServerInterceptor, finalSize)
	copy(mergedHandlers, s.handlers)
	copy(mergedHandlers[len(s.handlers):], handlers)
	s.handlers = mergedHandlers
	return s
}

// Endpoint return a real address to registry endpoint.
// examples:
//
//	tcp://127.0.0.1:6000?isSecure=false
func (s *Server) Endpoint() (*url.URL, error) {
	if err := s.listenAndEndpoint(); err != nil {
		return nil, s.err
	}
	return s.endpoint, nil
}

func (s *Server) RegisterService(sd *ServiceDesc, ss interface{}) (cl *ChanList) {
	ht := reflect.TypeOf(sd.HandlerType).Elem()
	st := reflect.TypeOf(ss)
	if !st.Implements(ht) {
		log.Fatalf("tcp: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
	}
	if s.m != nil {
		log.Fatalf("grpc: Server.RegisterService found duplicate service registration for %q", sd.ServiceName)
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
	cl = &ChanList{
		PushChan:       s.pushChan,
		CloseChan:      s.closeChan,
		DisconnectChan: s.disconnectChan,
	}
	//close
	go func() {
		for {
			mid := <-s.closeChan
			if channel := s.GetBucket(mid).Channel(mid); channel != nil {
				channel.Close()
			}
		}
	}()
	//push
	go func() {
		for {
			pd := <-s.pushChan
			s.PushByChannel(pd.Mid, pd.Ops, pd.Data)
		}
	}()
	return
}

func (s *Server) PushByChannel(sessionID string, ops int32, data []byte) {
	if channel := s.GetBucket(sessionID).Channel(sessionID); channel != nil {
		pushBody := &proto.Body{
			Ops:  ops,
			Data: data,
		}
		var (
			data []byte
			err  error
		)
		if data, err = gproto.Marshal(pushBody); err != nil {
			log.Errorf("push proto Marshal err:%+v", err)
			return
		}
		p := &proto.Payload{
			Place: 1,
			Type:  int32(proto.Push),
			Body:  data,
		}
		if err := channel.Push(p); err != nil {
			log.Errorf("channle push err：%+v", err)
		}
	}
}

func (s *Server) Operate(ctx context.Context, p *proto.Payload) (err error) {
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
		return
	}
	reply, errCode := md.Handler(srv.server, ctx, reqBody.Data, s.interceptor)
	if errCode != nil {
		log.Errorf("errCord=%+v p:%+v", errCode, p)
	}
	p.Code = 0 //int32(ecode.Cause(errCode).Code())
	p.Body = reply
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
