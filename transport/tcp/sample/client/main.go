package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	transhttp "github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/tcp"
	v1 "github.com/yola1107/kratos/v2/transport/tcp/sample/api/helloworld/v1"
	//"github.com/gogo/protobuf/proto"
	"google.golang.org/protobuf/proto"
)

func main() {
	connHTTP, err := transhttp.NewClient(
		context.Background(),
		transhttp.WithMiddleware(
			recovery.Recovery(),
		),
		transhttp.WithEndpoint("127.0.0.1:8000"),
	)
	if err != nil {
		panic(err)
	}
	defer connHTTP.Close()

	for {
		//callHTTP(connHTTP)
		callTCP()

		return
		time.Sleep(time.Second * 10000)
	}
}

func callHTTP(connHTTP *transhttp.Client) {
	client := v1.NewGreeterHTTPClient(connHTTP)
	reply, err := client.SayHelloReq(context.Background(), &v1.HelloRequest{Name: "kratos"})
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("[http] SayHello %s", reply.Message)
}

var id = int64(0)

func callTCP() {

	id++
	u := &user{
		ID:     id,
		game:   nil,
		pushes: nil,
		respes: nil,
	}
	u.pushes = map[int32]tcp.PushMsgHandle{}
	u.respes = map[int32]tcp.RespMsgHandle{
		int32(v1.GameCommand_SayHelloRsp):  u.OnSayHelloRsp,
		int32(v1.GameCommand_SayHello2Rsp): u.OnSayHello2Rsp,
	}

	connTCP, err := tcp.NewTcpClient(&tcp.ClientConfig{
		Addr:           ":6000",
		PushHandlers:   u.pushes,
		RespHandlers:   u.respes,
		DisconnectFunc: func() { log.Infof("DisconnectFunc %d", u.ID) },
		Token:          "",
	})
	if err != nil {
		panic(err)
	}
	u.game = connTCP
	defer u.game.Close()

	u.OnSayHello2Req()

	log.Infof("[TCP] SayHello %v", u.ID)
}

type user struct {
	ID int64

	game *tcp.Client

	pushes map[int32]tcp.PushMsgHandle
	respes map[int32]tcp.RespMsgHandle
}

func (u *user) OnSayHello2Req() {
	//req := v1.HelloRequest{Name: "aaa"}

	//if err := json.Unmarshal([]byte(login), req); err != nil {
	//	log.Fatal("req failed. %v", err)
	//}

	//aaa [10 3 97 97 97]
	//s := ToJSON(v1.HelloRequest{Name: "aaa"})
	//req := v1.HelloRequest{}
	//if err := json.Unmarshal([]byte(s), &req); err != nil {
	//	log.Fatal("req failed. %v", err)
	//}

	{
		req := v1.Hello2Request{Name: "aaa"}
		d, err := proto.Marshal(&req)
		if err != nil {
			log.Fatal("req failed. %v", err)
		}
		log.Infof("%s %+v", string(d), d)

		tmp := v1.Hello2Request{}
		if proto.Unmarshal(d, &tmp) != nil {
			log.Fatal("req failed. %v", err)
		}
	}

	req := v1.Hello2Request{Name: "aaa"}
	_ = u.game.Request(int32(v1.GameCommand_SayHello2Req), &req)
}

func (u *user) OnSayHelloRsp(data []byte, code int32) {
	log.Infof("[TCP] OnSayHelloRsp %d", code)
}

func (u *user) OnSayHello2Rsp(data []byte, code int32) {
	log.Infof("[TCP] OnSayHello2Rsp %d", code)
}

func ToJSON(v interface{}) string {
	j, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(j)
}
