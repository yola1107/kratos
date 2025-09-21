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
	doFunc := func(ctx context.Context, req *{{.Request}}) ([]byte, error) {
		doRequest := func() ([]byte, error) {
			resp, err := srv.({{$svrType}}WebsocketServer).{{.Name}}(ctx, req)
			if err != nil || resp == nil {
				return nil, err
			}
			return proto.Marshal(resp)
		}
		if loop := srv.({{$svrType}}WebsocketServer).GetLoop(); loop != nil {
			return loop.PostAndWaitCtx(ctx, doRequest)
		}
		return doRequest()
	}
	if interceptor == nil {
		return doFunc(ctx, in)
	}
	info := &websocket.UnaryServerInfo{
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
