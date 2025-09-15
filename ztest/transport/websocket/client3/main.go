package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	wproto "github.com/yola1107/kratos/v2/transport/websocket/proto"
	v1 "github.com/yola1107/kratos/v2/ztest/transport/api/helloworld/v1"
	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"
)

var (
	url      = flag.String("url", "ws://127.0.0.1:3102/", "WebSocket URL")
	conns    = flag.Int("c", 1000, "Number of concurrent connections")
	totalMsg = flag.Int("n", 1000, "Total messages per connection")
	batch    = flag.Int("b", 100, "Connections per batch")
)

// GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o client main.go
// ./client -url "ws://127.0.0.1:3102/" -c 10000 -n 1000 -b 100

func main() {
	flag.Parse()

	var (
		successCount int64
		connectFail  int64
		successConn  int64
		wg           sync.WaitGroup
		connTimeout  = 10 * time.Second
		msgTimeout   = 5 * time.Second
	)

	start := time.Now()
	log.Printf("Starting pressure test: %d conns * %d msg/conn = %d total\n", *conns, *totalMsg, *conns**totalMsg)

	// 分批次建立连接
	for i := 0; i < *conns; i++ {
		if i%*batch == 0 {
			time.Sleep(10 * time.Millisecond) // 控制连接速率
		}
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), connTimeout)
			defer cancel()

			conn, _, err := websocket.Dial(ctx, *url, nil)
			if err != nil {
				log.Printf("conn[%d] dial error: %v\n", id, err)
				atomic.AddInt64(&connectFail, 1)
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "done")
			atomic.AddInt64(&successConn, 1)

			for seq := 0; seq < *totalMsg; seq++ {
				msg := createMessage(id, seq)
				if err := sendMessage(conn, msg, msgTimeout); err != nil {
					log.Printf("conn[%d] error: %v", id, err)
					return
				}
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	log.Println("========== Result ==========")
	log.Printf("Connected successfully: %d / %d", successConn, *conns)
	log.Printf("Connection failures: %d", connectFail)
	log.Printf("Total messages sent: %d", successCount)
	log.Printf("Total time: %v", elapsed)
	log.Printf("QPS: %.2f", float64(successCount)/elapsed.Seconds())
}

func createMessage(id, seq int) []byte {
	hello2Req := &v1.Hello2Request{
		Name: fmt.Sprintf("client-%d", id),
		Seq:  int32(seq),
	}
	body, _ := proto.Marshal(hello2Req)
	wsMsg := &wproto.Payload{
		Op:      wproto.OpRequest,
		Seq:     int32(seq),
		Command: int32(v1.GameCommand_SayHello2Req),
		Body:    body,
	}
	msgData, _ := proto.Marshal(wsMsg)
	return msgData
}

func sendMessage(conn *websocket.Conn, msg []byte, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := conn.Write(ctx, websocket.MessageBinary, msg); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	_, _, err := conn.Read(ctx)
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}

	return nil
}
