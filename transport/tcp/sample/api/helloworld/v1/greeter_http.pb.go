// Code generated by protoc-gen-go-http. DO NOT EDIT.
// versions:
// - protoc-gen-go-http v2.8.3
// - protoc             v3.6.1
// source: helloworld/v1/greeter.proto

package v1

import (
	context "context"
	http "github.com/yola1107/kratos/v2/transport/http"
	binding "github.com/yola1107/kratos/v2/transport/http/binding"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the kratos package it is being compiled against.
var _ = new(context.Context)
var _ = binding.EncodeURL

const _ = http.SupportPackageIsVersion1

const OperationGreeterSayHello2Req = "/helloworld.v1.Greeter/SayHello2Req"
const OperationGreeterSayHelloReq = "/helloworld.v1.Greeter/SayHelloReq"

type GreeterHTTPServer interface {
	SayHello2Req(context.Context, *Hello2Request) (*Hello2Reply, error)
	// SayHelloReq Sends a greeting
	SayHelloReq(context.Context, *HelloRequest) (*HelloReply, error)
}

func RegisterGreeterHTTPServer(s *http.Server, srv GreeterHTTPServer) {
	r := s.Route("/")
	r.GET("/helloworld/{name}", _Greeter_SayHelloReq0_HTTP_Handler(srv))
	r.GET("/helloworld2/{name}", _Greeter_SayHello2Req0_HTTP_Handler(srv))
}

func _Greeter_SayHelloReq0_HTTP_Handler(srv GreeterHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in HelloRequest
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		if err := ctx.BindVars(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationGreeterSayHelloReq)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.SayHelloReq(ctx, req.(*HelloRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*HelloReply)
		return ctx.Result(200, reply)
	}
}

func _Greeter_SayHello2Req0_HTTP_Handler(srv GreeterHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in Hello2Request
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		if err := ctx.BindVars(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationGreeterSayHello2Req)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.SayHello2Req(ctx, req.(*Hello2Request))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*Hello2Reply)
		return ctx.Result(200, reply)
	}
}

type GreeterHTTPClient interface {
	SayHello2Req(ctx context.Context, req *Hello2Request, opts ...http.CallOption) (rsp *Hello2Reply, err error)
	SayHelloReq(ctx context.Context, req *HelloRequest, opts ...http.CallOption) (rsp *HelloReply, err error)
}

type GreeterHTTPClientImpl struct {
	cc *http.Client
}

func NewGreeterHTTPClient(client *http.Client) GreeterHTTPClient {
	return &GreeterHTTPClientImpl{client}
}

func (c *GreeterHTTPClientImpl) SayHello2Req(ctx context.Context, in *Hello2Request, opts ...http.CallOption) (*Hello2Reply, error) {
	var out Hello2Reply
	pattern := "/helloworld2/{name}"
	path := binding.EncodeURL(pattern, in, true)
	opts = append(opts, http.Operation(OperationGreeterSayHello2Req))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "GET", path, nil, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *GreeterHTTPClientImpl) SayHelloReq(ctx context.Context, in *HelloRequest, opts ...http.CallOption) (*HelloReply, error) {
	var out HelloReply
	pattern := "/helloworld/{name}"
	path := binding.EncodeURL(pattern, in, true)
	opts = append(opts, http.Operation(OperationGreeterSayHelloReq))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "GET", path, nil, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
