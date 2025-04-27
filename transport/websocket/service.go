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

	connectFunc    func(*Session) // 连接建立回调
	disconnectFunc func(*Session) // 连接关闭回调
}

type MethodDesc struct {
	Ops        int32
	MethodName string
	Handler    methodHandler
}

type methodHandler func(srv interface{}, ctx context.Context, data []byte, interceptor UnaryServerInterceptor) ([]byte, error)

func (s *Server) RegisterService(sd *ServiceDesc, ss interface{}, onOpen, onClose func(session *Session)) {
	ht := reflect.TypeOf(sd.HandlerType).Elem()
	st := reflect.TypeOf(ss)
	if !st.Implements(ht) {
		log.Fatalf("websocket: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
	}
	if s.m != nil {
		log.Fatalf("websocket: Server.RegisterService found duplicate service registration for %q", sd.ServiceName)
	}
	srv := &service{
		server: ss,
		md:     make(map[int32]*MethodDesc),

		connectFunc:    onOpen,
		disconnectFunc: onClose,
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		srv.md[d.Ops] = d
	}
	s.m = srv
}
