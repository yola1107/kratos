package gnet

import (
	"context"
	"reflect"

	"github.com/yola1107/kratos/v2/log"
)

// ServiceDesc describes a gnet service and its methods.
type ServiceDesc struct {
	ServiceName string
	HandlerType interface{}
	Methods     []MethodDesc
}

// MethodDesc describes a gnet method.
type MethodDesc struct {
	Ops        int32
	MethodName string
	Handler    methodHandler
}

type methodHandler func(srv interface{}, ctx context.Context, data []byte, interceptor UnaryServerInterceptor) ([]byte, error)

type service struct {
	server interface{}
	name   string
	md     map[int32]*MethodDesc
}

func (s *service) register(sd *ServiceDesc, ss interface{}) {
	ht := reflect.TypeOf(sd.HandlerType).Elem()
	st := reflect.TypeOf(ss)
	if !st.Implements(ht) {
		log.Fatalf("gnet: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
	}
	if s.server != nil {
		log.Fatalf("gnet: Server.RegisterService found duplicate service registration for %q", sd.ServiceName)
	}
	s.server = ss
	s.name = sd.ServiceName
	for i := range sd.Methods {
		d := &sd.Methods[i]
		s.md[d.Ops] = d
	}
}

// UnaryServerInfo provides information about current call.
type UnaryServerInfo struct {
	Server     interface{}
	FullMethod string
}

// UnaryHandler defines handler that executes RPC.
type UnaryHandler func(ctx context.Context, req interface{}) ([]byte, error)

// UnaryServerInterceptor intercepts RPC execution on server side.
type UnaryServerInterceptor func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) ([]byte, error)
