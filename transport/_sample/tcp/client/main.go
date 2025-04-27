package main

import (
	"fmt"
	"time"

	pb "github.com/yola1107/kratos/v2/internal/testdata/helloworld"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/_sample/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/transport/tcp"
)

var (
	seed = int64(0)
)

func main() {

	addr := "0.0.0.0:3101"

	c, err := tcp.NewTcpClient(&tcp.ClientConfig{
		Addr: addr,
		PushHandlers: map[int32]tcp.PushMsgHandle{
			int32(v1.GameCommand_SayHelloRsp):  func(data []byte) { log.Infof("tcp-> 1002 cb. data=%+v", data) },
			int32(v1.GameCommand_SayHello2Rsp): func(data []byte) { log.Infof("tcp-> 1004 cb. data=%+v", data) },
		},
		RespHandlers: map[int32]tcp.RespMsgHandle{
			int32(v1.GameCommand_SayHelloReq):  func(data []byte, code int32) { log.Infof("tcp-> 1001 req. data=%+v code=%d", data, code) },
			int32(v1.GameCommand_SayHello2Req): func(data []byte, code int32) { log.Infof("tcp-> 1003 req. data=%+v code=%d", data, code) },
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
		req := pb.HelloRequest{Name: fmt.Sprintf("tcp_%d", i)}
		if err = c.Request(int32(v1.GameCommand_SayHelloReq), &req); err != nil {
			panic(err)
		}
		i++
		if i > 65535 {
			i = 0
		}
		time.Sleep(time.Second * 10)
	}
}
