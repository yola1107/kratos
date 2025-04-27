package main

import (
	"context"
	"fmt"
	"time"

	"github.com/yola1107/kratos/contrib/log/zap/v2"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	"github.com/yola1107/kratos/v2/transport/_sample/api/helloworld/v1"
	transgrpc "github.com/yola1107/kratos/v2/transport/grpc"
	transhttp "github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/websocket"
	"google.golang.org/grpc"

	"github.com/yola1107/kratos/contrib/registry/etcd/v2"
	etcdv3 "go.etcd.io/etcd/client/v3"
)

var (
	seed = int64(0)
)

func main() {

	//// 生产环境配置
	//zapLogger := zap.New(&zap.Options{
	//	Mode:          zap.Production, // os.Getenv("APP_ENV")
	//	Level:         "debug",
	//	Directory:     "./logs",
	//	Filename:      "client.log",
	//	ErrorFilename: "client-error.log",
	//	MaxSize:       500,
	//	MaxAge:        30,
	//})
	//defer zapLogger.Close()

	zapLogger := zap.New(nil)
	defer zapLogger.Close()

	log.SetLogger(zapLogger)
	log.Infof("start clients.")
	defer log.Infof("stop clients.")

	etcdClient, err := etcdv3.New(etcdv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	})
	if err != nil {
		panic(err)
	}
	r := etcd.New(etcdClient)

	//http
	connHTTP, err := transhttp.NewClient(
		context.Background(),
		//transhttp.WithEndpoint("127.0.0.1:8000"),
		transhttp.WithEndpoint("discovery:///helloworld"),
		transhttp.WithDiscovery(r),
		transhttp.WithMiddleware(
			recovery.Recovery(),
		),
	)
	if err != nil {
		panic(err)
	}
	defer connHTTP.Close()

	// grpc
	connGRPC, err := transgrpc.DialInsecure(
		context.Background(),
		//transgrpc.WithEndpoint("127.0.0.1:9000"),
		transgrpc.WithEndpoint("discovery:///helloworld"),
		transgrpc.WithDiscovery(r),
		transgrpc.WithMiddleware(
			recovery.Recovery(),
		),
	)
	if err != nil {
		panic(err)
	}
	defer connGRPC.Close()

	//websocket 不使用服务注册发现功能
	wsClient, err := websocket.NewClient(
		context.Background(),
		//websocket.WithEndpoint("discovery:///helloworld"),
		//websocket.WithDiscovery(r),
		websocket.WithEndpoint("127.0.0.1:3102"),
		websocket.WithToken(""),
		websocket.WithPushHandler(map[int32]websocket.PushHandler{
			int32(v1.GameCommand_SayHelloRsp):  func(data []byte) { log.Infof("ws-> 1002 cb. %v", data) },
			int32(v1.GameCommand_SayHello2Rsp): func(data []byte) { log.Infof("ws-> 1003 cb. %v", data) },
		}),
		websocket.WithResponseHandler(map[int32]websocket.ResponseHandler{
			int32(v1.GameCommand_SayHelloReq):  func(data []byte, code int32) { log.Infof("ws-> 1001. data=%+v code=%d", data, code) },
			int32(v1.GameCommand_SayHello2Req): func(data []byte, code int32) { log.Infof("ws-> 1013. data=%+v code=%d", data, code) },
			int32(6666):                        func(data []byte, code int32) { log.Infof("ws-> 6666. data=%+v code=%d", data, code) },
			int32(9999):                        func(data []byte, code int32) { log.Infof("ws-> 9999. data=%+v code=%d", data, code) },
		}),
		websocket.WithDisconnectFunc(func() { log.Infof("disconnect called") }),
		websocket.WithStateFunc(func(connected bool) { log.Infof("连接状态变更. connectd=%+v", connected) }),
	)
	if err != nil {
		panic(err)
	}
	defer wsClient.Close()

	for {
		if wsClient.Closed() {
			break
		}
		seed++
		callHTTP(connHTTP)
		callGRPC(connGRPC)
		callWebsocket(wsClient)
		//time.Sleep(time.Millisecond * 20)
		time.Sleep(time.Second * 10)
	}
}

func callHTTP(connHTTP *transhttp.Client) {
	client := v1.NewGreeterHTTPClient(connHTTP)
	reply, err := client.SayHelloReq(context.Background(), &v1.HelloRequest{Name: "kratos_http"})
	if err != nil {
		log.Errorf("err:%+v", err)
	} else {
		log.Infof("[http] SayHello %s", reply.Message)
	}
}

func callGRPC(connGRPC *grpc.ClientConn) {
	client := v1.NewGreeterClient(connGRPC)
	reply, err := client.SayHelloReq(context.Background(), &v1.HelloRequest{Name: "kratos_grpc"})
	if err != nil {
		log.Errorf("err:%+v", err)
	} else {
		log.Infof("[grpc] SayHello %s", reply.Message)
	}
}

func callWebsocket(c *websocket.Client) {
	if _, err := c.Request(int32(v1.GameCommand_SayHello2Req), &v1.Hello2Request{Name: fmt.Sprintf("ws:%d", seed)}); err != nil {
		log.Errorf("[ws] %+v", err)
	}
	if _, err := c.Request(6666, &v1.Hello2Request{Name: fmt.Sprintf("ws:%d", seed)}); err != nil {
		log.Errorf("[ws] %+v", err)
	}
	if _, err := c.Request(9999, &v1.HelloRequest{Name: fmt.Sprintf("ws:%d", seed)}); err != nil {
		log.Errorf("[ws] %+v", err)
	}
}
