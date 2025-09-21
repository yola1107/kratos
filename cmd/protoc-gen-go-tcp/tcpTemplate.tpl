{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}

// {{$svrType}}TCPServer is the server API for {{$svrType}} service.
type {{$svrType}}TCPServer interface {
	GetTCPLoop() work.Loop
	SetCometChan(cl *tcp.ChanList, cs *tcp.Server)
{{- range .Methods}}
	{{- if ne .Comment ""}}
	{{.Comment}}
	{{- end}}
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{- end}}
}

func Register{{$svrType}}TCPServer(s *tcp.Server, srv {{$svrType}}TCPServer) {
	chanList := s.RegisterService(&{{$svrType}}_TCP_ServiceDesc, srv)
	srv.SetCometChan(chanList, s)
}

{{range .Methods}}
func _{{$svrType}}_{{.Name}}_TCP_Handler(srv interface{}, ctx context.Context, data []byte, interceptor tcp.UnaryServerInterceptor) ([]byte, error) {
	in := new({{.Request}})
	if err := proto.Unmarshal(data, in); err != nil {
		return nil, err
	}
	doFunc := func(ctx context.Context, req *{{.Request}}) ([]byte, error) {
		doRequest := func() ([]byte, error) {
			resp, err := srv.({{$svrType}}TCPServer).{{.Name}}(ctx, req)
			if err != nil || resp == nil {
				return nil, err
			}
			return proto.Marshal(resp)
		}
		if loop := srv.({{$svrType}}TCPServer).GetTCPLoop(); loop != nil {
			return loop.PostAndWaitCtx(ctx, doRequest)
		}
		return doRequest()
	}
	if interceptor == nil {
		return doFunc(ctx, in)
	}
	info := &tcp.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/{{$svrName}}/{{.OriginalName}}",
	}
	interceptorHandler := func(ctx context.Context, req interface{}) ([]byte, error) {
		r, ok := req.(*{{.Request}})
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid Request Argument, expect: *{{.Request}}, Not: %T", req)
		}
		return doFunc(ctx, r)
	}
	return interceptor(ctx, in, info, interceptorHandler)
}
{{end}}

var {{$svrType}}_TCP_ServiceDesc = tcp.ServiceDesc{
	ServiceName: "{{$svrName}}",
	HandlerType: (*{{$svrType}}TCPServer)(nil),
	Methods: []tcp.MethodDesc{
		{{- range .Methods}}
		{
			MethodName: "{{.OriginalName}}",
			Handler:    _{{$svrType}}_{{.Name}}_TCP_Handler,
			Ops:        {{.Ops}},
		},
		{{- end}}
	},
}
