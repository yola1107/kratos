package main

import (
	"context"
	"fmt"
	"time"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/log/zap/conf"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	transgrpc "github.com/yola1107/kratos/v2/transport/grpc"
	transhttp "github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/websocket"
	wproto "github.com/yola1107/kratos/v2/transport/websocket/proto"
	v1 "github.com/yola1107/kratos/v2/ztest/transport/api/helloworld/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/yola1107/kratos/v2/library/log/zap"
	// "github.com/yola1107/kratos/contrib/registry/etcd/v2"
	// etcdv3 "go.etcd.io/etcd/client/v3"
)

var (
	seed = int64(0)
)

func main() {
	Name := "ws-client"
	zapLogger := zap.NewLogger(conf.DefaultConfig(
		// conf.WithProduction(),
		conf.WithAppName(Name),
		conf.WithLevel("debug"),
		conf.WithDirectory("./logs"),
		conf.WithAlertEnable(true),
		conf.WithTelegramToken("7945687310:AAHA9tkUPV1ELEsVSLoDZe_Cc76wp7YdDVI"),
		conf.WithTelegramChatID("-4672893880"),
	))
	defer zapLogger.Close()

	log.SetLogger(zapLogger)

	log.Infof("start clients.")
	defer log.Infof("stop clients.")
	//
	// etcdClient, err := etcdv3.New(etcdv3.Config{
	//	Endpoints: []string{"127.0.0.1:2379"},
	// })
	// if err != nil {
	//	panic(err)
	// }
	// r := etcd.New(etcdClient)

	// http
	connHTTP, err := transhttp.NewClient(
		context.Background(),
		transhttp.WithEndpoint("127.0.0.1:8000"),
		// transhttp.WithEndpoint("discovery:///helloworld"),
		// transhttp.WithDiscovery(r),
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
		transgrpc.WithEndpoint("127.0.0.1:9000"),
		// transgrpc.WithEndpoint("discovery:///helloworld"),
		// transgrpc.WithDiscovery(r),
		transgrpc.WithMiddleware(
			recovery.Recovery(),
		),
	)
	if err != nil {
		panic(err)
	}
	defer connGRPC.Close()

	// websocket 不使用服务注册发现功能
	wsClient, err := websocket.NewClient(
		context.Background(),
		// websocket.WithEndpoint("discovery:///helloworld"),
		// websocket.WithDiscovery(r),
		websocket.WithEndpoint("127.0.0.1:3102"),
		websocket.WithToken(""),
		websocket.WithPushHandler(map[int32]websocket.PushHandler{
			int32(v1.GameCommand_SayHelloRsp):  func(data []byte) { log.Infof("PushHandler(1002). data=%v", data) },
			int32(v1.GameCommand_SayHello2Rsp): func(data []byte) { log.Infof("PushHandler(1004). data=%v", unmarshalProtoMsg(data)) },
		}),
		websocket.WithResponseHandler(map[int32]websocket.ResponseHandler{
			int32(v1.GameCommand_SayHelloReq): func(data []byte, code int32) {}, // 空
			int32(v1.GameCommand_SayHello2Req): func(data []byte, code int32) {
				log.Infof("respHanler(1003) code=%d data=%v", code, unmarshalProtoMsg(data))
			}, // 空
		}),
		websocket.WithConnectFunc(func(session *websocket.Session) { log.Infof("connect called. %q", session.ID()) }),
		websocket.WithDisconnectFunc(func(session *websocket.Session) { log.Infof("disconnect called. %q", session.ID()) }),
	)
	if err != nil {
		log.Warnf("connect to server failed. %+v", err)
		return
	}
	defer wsClient.Close()

	// can := wsClient.CanRetry()
	for wsClient.IsAlive() {
		seed++
		callHTTP(connHTTP)
		callGRPC(connGRPC)
		callWebsocket(wsClient)
		time.Sleep(time.Millisecond * 5000)
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
	if sess := c.GetSession(); sess == nil || sess.Closed() {
		return
	}
	if err := c.Request(int32(v1.GameCommand_SayHello2Req), &v1.Hello2Request{Name: fmt.Sprintf("kratos_ws:%d", seed)}); err != nil {
		log.Errorf("err:%+v", err)
	}

	// payload, err := c.Request(int32(v1.GameCommand_SayHello2Req), &v1.Hello2Request{Name: fmt.Sprintf("kratos_ws:%d", seed)})
	// if err != nil || payload == nil {
	// 	log.Errorf("err:%+v payload:%+v", err, descPayload(payload))
	// } else {
	// 	log.Infof("[ws] Request recv payload. %s", descPayload(payload))
	// }
}

func unmarshalProtoMsg(data []byte) string {
	resp := v1.Hello2Reply{}
	if err := proto.Unmarshal(data, &resp); err != nil {
		log.Errorf("err:%+v", err)
		return fmt.Sprintf("err:%+v", err)
	}
	return fmt.Sprintf("%+v", ext.ToJSON(&resp))
}

func descPayload(p *wproto.Payload) string {
	if p == nil {
		return ""
	}
	body := v1.Hello2Reply{}
	if len(p.Body) > 0 {
		_ = proto.Unmarshal(p.Body, &body)
	}
	return fmt.Sprintf("{op=%v place=%d seq=%v code=%v command=%d body=%+v",
		p.Op, p.Place, p.Seq, p.Code, p.Command, ext.ToJSON(&body))
}
