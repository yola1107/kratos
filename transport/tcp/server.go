package tcp

import (
	"context"
	"net/url"
	"time"

	"github.com/yola1107/kratos/v2/transport"
)

var (
	_ transport.Server     = (*Server)(nil)
	_ transport.Endpointer = (*Server)(nil)
)

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

type PushData struct {
	Mid  string
	Ops  int32
	Data []byte
}

type ChanList struct {
	PushChan       chan *PushData
	CloseChan      chan string
	DisconnectChan chan string
}

type ServiceDesc struct {
	ServiceName string
	// The pointer to the service interface. Used to check whether the user
	// provided implementation satisfies the interface requirements.
	HandlerType interface{}
	Methods     []MethodDesc
}

type MethodDesc struct {
	Ops        int32
	MethodName string
	Handler    methodHandler
}

type methodHandler func(srv interface{}, ctx context.Context, data []byte, interceptor UnaryServerInterceptor) ([]byte, error)

// Server is an TCP server wrapper.
type Server struct {
	network  string
	address  string
	endpoint *url.URL
	timeout  time.Duration
}

// NewServer creates an TCP server by options.
func NewServer(opts ...ServerOption) *Server {
	return &Server{}
}

func (s *Server) Start(context.Context) error {
	return nil
}

func (s *Server) Stop(context.Context) error {
	return nil
}

func (s *Server) Endpoint() (*url.URL, error) {
	return nil, nil
}

func (s *Server) RegisterService(sd *ServiceDesc, ss interface{}) (cl *ChanList) {
	return nil
}
