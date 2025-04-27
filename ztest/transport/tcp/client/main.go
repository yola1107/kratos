package main

import (
	"fmt"
	"time"

	pb "github.com/yola1107/kratos/v2/internal/testdata/helloworld"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/library/log/zap/conf"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/tcp"
	"github.com/yola1107/kratos/v2/ztest/transport/api/helloworld/v1"
	"google.golang.org/protobuf/proto"
)

var (
	seed = int64(0)
)

func main() {
	logger := zap.NewLogger(conf.DefaultConfig(
		conf.WithAppName("tcp-client"),
	))
	defer logger.Close()
	log.SetLogger(logger)

	addr := "0.0.0.0:3101"

	c, err := tcp.NewTcpClient(&tcp.ClientConfig{
		Addr: addr,
		PushHandlers: map[int32]tcp.PushMsgHandle{
			int32(v1.GameCommand_SayHelloRsp):  func(data []byte) { log.Infof("PushMsgHandle(1002). data=%+v", data) },
			int32(v1.GameCommand_SayHello2Rsp): func(data []byte) { log.Infof("PushMsgHandle(1004). data=%+v", unmarshalProtoMsg(data)) },
		},
		RespHandlers: map[int32]tcp.RespMsgHandle{
			int32(v1.GameCommand_SayHelloReq):  func(data []byte, code int32) { log.Infof("RespHandlers(1001). code=%d data=%+v ", code, data) },
			int32(v1.GameCommand_SayHello2Req): func(data []byte, code int32) { log.Infof("RespHandlers(1003). code=%d data=%+v ", code, data) },
		},
		DisconnectFunc: func() { log.Infof("disconect.") },
		Token:          "",
	})
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// 向tcp服务器发请求
	i := 0
	for {
		req := pb.HelloRequest{Name: fmt.Sprintf("kratos_tcp_%d", i)}
		if err = c.Request(int32(v1.GameCommand_SayHello2Req), &req); err != nil {
			log.Errorf("tcp request err: %v", err)
		}
		i++
		if i > 65535 {
			i = 0
		}
		time.Sleep(time.Millisecond * 10000)
	}
}

func unmarshalProtoMsg(data []byte) string {
	resp := v1.Hello2Reply{}
	if err := proto.Unmarshal(data, &resp); err != nil {
		log.Errorf("err:%+v", err)
		return fmt.Sprintf("err:%+v", err)
	}
	return fmt.Sprintf("%+v", ext.ToJSON(&resp))
}
