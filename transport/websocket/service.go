package websocket

import (
	"context"
	"reflect"

	"github.com/yola1107/kratos/v2/log"
)

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

type MethodDesc struct {
	Ops        int32
	MethodName string
	Handler    methodHandler
}

type methodHandler func(srv interface{}, ctx context.Context, data []byte, interceptor UnaryServerInterceptor) ([]byte, error)

func (s *Server) RegisterService(sd *ServiceDesc, ss interface{}) {
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
}
