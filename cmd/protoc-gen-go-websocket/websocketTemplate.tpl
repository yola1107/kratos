{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}

// {{$svrType}}WebsocketServer is the server API for {{$svrType}} service.
type {{$svrType}}WebsocketServer interface {
	GetLoop() work.Loop
	OnSessionOpen(*websocket.Session)
	OnSessionClose(*websocket.Session)
{{- range .Methods}}
	{{- if ne .Comment ""}}
	{{.Comment}}
	{{- end}}
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{- end}}
}

func Register{{$svrType}}WebsocketServer(s *websocket.Server, srv {{$svrType}}WebsocketServer) {
	s.RegisterService(&{{$svrType}}_Websocket_ServiceDesc, srv, srv.OnSessionOpen, srv.OnSessionClose)
}

{{range .Methods}}
func _{{$svrType}}_{{.Name}}_Websocket_Handler(srv interface{}, ctx context.Context, data []byte, interceptor websocket.UnaryServerInterceptor) ([]byte, error) {
	in := new({{.Request}})
	if err := proto.Unmarshal(data, in); err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, req *{{.Request}}) ([]byte, error) {
		resp, err := srv.({{$svrType}}WebsocketServer).{{.Name}}(ctx, req)
		if err != nil {
			return nil, err
		}
		data, err := proto.Marshal(resp)
		if err != nil {
			return nil, err
		}
		if loop := srv.({{$svrType}}WebsocketServer).GetLoop(); loop != nil {
			return loop.PostAndWaitCtx(ctx, func() ([]byte, error) { return data, nil })
		}
		return data, nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	info := &websocket.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/{{$svrName}}/{{.OriginalName}}",
	}
	interceptorHandler := func(ctx context.Context, req interface{}) ([]byte, error) {
		r, ok := req.(*{{.Request}})
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid Request Argument, expect: *{{.Request}}, Not: %T", req)
		}
		return handler(ctx, r)
	}
	return interceptor(ctx, in, info, interceptorHandler)
}
{{end}}

var {{$svrType}}_Websocket_ServiceDesc = websocket.ServiceDesc{
	ServiceName: "{{$svrName}}",
	HandlerType: (*{{$svrType}}WebsocketServer)(nil),
	Methods: []websocket.MethodDesc{
		{{- range .Methods}}
		{
			MethodName: "{{.OriginalName}}",
			Handler:    _{{$svrType}}_{{.Name}}_Websocket_Handler,
			Ops:        {{.Ops}},
		},
		{{- end}}
	},
}
