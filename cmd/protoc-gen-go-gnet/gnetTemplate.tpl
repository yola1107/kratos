{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}

// {{$svrType}}GNETServer is the server API for {{$svrType}} service.
type {{$svrType}}GNETServer interface {
	GetLoop() work.Loop
{{- range .Methods}}
	{{- if ne .Comment ""}}
	{{.Comment}}
	{{- end}}
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{- end}}
}

func Register{{$svrType}}GNETServer(s *gnet.Server, srv {{$svrType}}GNETServer) {
	s.RegisterService(&{{$svrType}}_GNET_ServiceDesc, srv)
}

{{range .Methods}}
func _{{$svrType}}_{{.Name}}_GNET_Handler(srv interface{}, ctx context.Context, data []byte, interceptor gnet.UnaryServerInterceptor) ([]byte, error) {
	in := new({{.Request}})
	if err := proto.Unmarshal(data, in); err != nil {
		return nil, err
	}
	doFunc := func(ctx context.Context, req *{{.Request}}) ([]byte, error) {
		doRequest := func() ([]byte, error) {
			resp, err := srv.({{$svrType}}GNETServer).{{.Name}}(ctx, req)
			if err != nil || resp == nil {
				return nil, err
			}
			return proto.Marshal(resp)
		}
		if loop := srv.({{$svrType}}GNETServer).GetLoop(); loop != nil {
			return loop.PostAndWaitCtx(ctx, doRequest)
		}
		return doRequest()
	}
	if interceptor == nil {
		return doFunc(ctx, in)
	}
	info := &gnet.UnaryServerInfo{
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

var {{$svrType}}_GNET_ServiceDesc = gnet.ServiceDesc{
	ServiceName: "{{$svrName}}",
	HandlerType: (*{{$svrType}}GNETServer)(nil),
	Methods: []gnet.MethodDesc{
		{{- range .Methods}}
		{
			MethodName: "{{.OriginalName}}",
			Handler:    _{{$svrType}}_{{.Name}}_GNET_Handler,
			Ops:        {{.Ops}},
		},
		{{- end}}
	},
}
