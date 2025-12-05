package main

import (
	"context"
	"fmt"
	"time"

	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/library/log/zap/conf"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/gnet"
	v1 "github.com/yola1107/kratos/v2/ztest/transport/api/helloworld/v1"
)

func main() {
	logger := zap.NewLogger(conf.DefaultConfig(
		conf.WithAppName("gnet-echo-client"),
	))
	defer logger.Close()
	log.SetLogger(logger)

	// 创建 gnet 客户端
	client := gnet.NewClient(
		gnet.WithEndpoint("127.0.0.1:3200"),
		gnet.WithTimeout(5*time.Second),
	)

	// 测试 SayHelloReq (Ops: 1001)
	log.Info("=== Testing SayHelloReq ===")
	for i := 0; i < 5; i++ {
		req := &v1.HelloRequest{
			Name: fmt.Sprintf("Hello from client %d", i),
		}

		var reply v1.HelloReply
		err := client.Invoke(context.Background(), 1001, req, &reply)
		if err != nil {
			log.Errorf("SayHelloReq failed: %v", err)
			continue
		}

		log.Infof("[gnet] SayHelloReq response: %s", reply.Message)
		time.Sleep(time.Second)
	}

	// 测试 SayHello2Req (Ops: 1003)
	log.Info("=== Testing SayHello2Req ===")
	for i := 0; i < 5; i++ {
		req := &v1.Hello2Request{
			Name: fmt.Sprintf("Hello2 from client %d", i),
			Seq:  int32(i),
		}

		var reply v1.Hello2Reply
		err := client.Invoke(context.Background(), 1003, req, &reply)
		if err != nil {
			log.Errorf("SayHello2Req failed: %v", err)
			continue
		}

		log.Infof("[gnet] SayHello2Req response: %s", reply.Message)
		time.Sleep(time.Second)
	}
}
