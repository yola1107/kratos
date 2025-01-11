{{/* 服务和方法定义 */}}
{{$svrType := .ServiceType}}               // 服务类型，如：Metadata
{{$svrName := .ServiceName}}               // 服务名称，如：kratos.api.Metadata

{{/* 生成操作名称常量 */}}
{{- range .MethodSets}}
const Operation{{$svrType}}{{.OriginalName}} = "/{{$svrName}}/{{.OriginalName}}"
{{- end}}

{{/* 定义TCP服务接口 */}}
type {{.ServiceType}}TCPServer interface {
{{- range .MethodSets}}
    {{- if .Comment}}
        // {{.Comment}}
    {{- end}}
    {{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{- end}}
}

{{/* 注册TCP服务 */}}
func Register{{.ServiceType}}TCPServer(s *tcp.Server, srv {{.ServiceType}}TCPServer) {
    chanList := s.RegisterService(&{{.ServiceType}}_TCP_ServiceDesc, srv)
    srv.SetCometChan(chanList, s)
    ins = &Loop{jobs: make(chan func(), 10000), toggle: make(chan byte)}
    ins.Start()
}

{{/* 定义每个方法的处理函数 */}}
{{range .Methods}}
func _{{$svrType}}_{{.Name}}{{.Num}}_TCP_Handler(srv interface{}, ctx context.Context, data []byte, interceptor tcp.UnaryServerInterceptor) ([]byte, error) {
    in := new({{.Request}})
    err := proto.Unmarshal(data, in)
    if err != nil {
        return nil, err
    }

    if interceptor == nil {
        out, err := srv.({{$svrType}}TCPServer).{{.Name}}(ctx, in)
        data, _ := proto.Marshal(out)
        return data, err
    }

    info := &tcp.UnaryServerInfo{
        Server:     srv,
        FullMethod: "/{{$svrName}}/{{.OriginalName}}",
    }
    handler := func(ctx context.Context, req interface{}) ([]byte, error) {
        out := new({{.Reply}})
        var err error
        if srv.({{$svrType}}TCPServer).IsLoopFunc("{{.Name}}") {
            rspChan := make(chan *{{.Reply}})
            errChan := make(chan error)
            ins.Post(func() {
                resp, err := srv.({{$svrType}}TCPServer).{{.Name}}(ctx, req.(*{{.Request}}))
                rspChan <- resp
                errChan <- err
            })
            out = <-rspChan
            err = <-errChan
        } else {
            out, err = srv.({{$svrType}}TCPServer).{{.Name}}(ctx, req.(*{{.Request}}))
        }
        if out != nil {
            data, _ := proto.Marshal(out)
            return data, err
        }
        return nil, err
    }

    return interceptor(ctx, in, info, handler)
}
{{end}}

{{/* 定义服务描述符 */}}
var {{.ServiceType}}_TCP_ServiceDesc = tcp.ServiceDesc{
    ServiceName: "{{$svrName}}",
    HandlerType: (*{{.ServiceType}}TCPServer)(nil),
    Methods: []tcp.MethodDesc{
        {{- range .MethodSets}}
        {
            MethodName: "{{.OriginalName}}",
            Handler:    _{{$svrType}}_{{.OriginalName}}{{.Num}}_TCP_Handler,
            Ops:         {{.Ops}},
        },
        {{- end}}
    },
}

{{/* 定义循环执行器，用于支持异步任务 */}}
type Loop struct {
    jobs   chan func()   // 任务队列
    toggle chan byte     // 控制任务循环的停止
}

func (lp *Loop) Start() {
    log.Info("loop routine start.")
    go func() {
        defer RecoverFromError(func() {
            lp.Start()
        })
        for {
            select {
            case <-lp.toggle:
                log.Info("Loop routine stop.")
                return
            case job := <-lp.jobs:
                job()   // 执行任务
            }
        }
    }()
}

func (lp *Loop) Stop() {
    go func() {
        lp.toggle <- 1
    }()
}

func Stop() { ins.Stop() }

func (lp *Loop) Jobs() int {
    return len(lp.jobs)
}

func Jobs() int { return ins.Jobs() }

func (lp *Loop) Post(job func()) {
    go func() {
        lp.jobs <- job
    }()
}

func Post(job func()) { ins.Post(job) }

func (lp *Loop) PostAndWait(job func() interface{}) interface{} {
    ch := make(chan interface{})
    go func() {
        lp.jobs <- func() {
            ch <- job()
        }
    }()
    return <-ch
}

func PostAndWait(job func() interface{}) interface{} { return ins.PostAndWait(job) }

