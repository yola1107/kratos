{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}

// {{$svrType}}GNETServer is the server API for {{$svrType}} service.
type {{$svrType}}GNETServer interface {
{{- range .MethodSets}}
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
		resp, err := srv.({{$svrType}}GNETServer).{{.Name}}(ctx, req)
		if err != nil || resp == nil {
			return nil, err
		}
		return proto.Marshal(resp)
	}
	if interceptor == nil {
		return doFunc(ctx, in)
	}
	info := &gnet.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/{{$svrName}}/{{.OriginalName}}",
	}
	handler := func(ctx context.Context, req interface{}) ([]byte, error) {
		r, ok := req.(*{{.Request}})
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid Request Argument, expect: *{{.Request}}, Not: %T", req)
		}
		return doFunc(ctx, r)
	}
	return interceptor(ctx, in, info, handler)
}
{{end}}

var {{$svrType}}_GNET_ServiceDesc = gnet.ServiceDesc{
	ServiceName: "{{$svrName}}",
	HandlerType: (*{{$svrType}}GNETServer)(nil),
	Methods: []gnet.MethodDesc{
		{{- range .MethodSets}}
		{
			MethodName: "{{.OriginalName}}",
			Handler:    _{{$svrType}}_{{.OriginalName}}_GNET_Handler,
			Ops:        {{.Ops}},
		},
		{{- end}}
	},
}

