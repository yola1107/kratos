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

// 使用现有的 HelloRequest 和 HelloReply 作为 Echo 消息

type EchoRequest = v1.HelloRequest
type EchoReply = v1.HelloReply

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

	// 发送 echo 请求
	for i := 0; i < 10; i++ {
		req := &EchoRequest{
			Name: fmt.Sprintf("Hello from client %d", i),
		}

		var reply EchoReply
		err := client.Invoke(context.Background(), 1001, req, &reply)
		if err != nil {
			log.Errorf("Echo request failed: %v", err)
			continue
		}

		log.Infof("[gnet] Echo response: %s", reply.Message)
		time.Sleep(time.Second)
	}
}
