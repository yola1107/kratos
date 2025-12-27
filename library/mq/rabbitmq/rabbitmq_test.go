package rabbitmq

import (
	"fmt"
	"log"
	"testing"
	"time"
)

func TestRabbitMQ_PublishConsume_Advanced(t *testing.T) {
	// 配置连接
	opts := Options{
		Host:     "192.168.152.128",
		Port:     "5672",
		Username: "admin",
		Password: "myMQ#@g3359Blue#@test.com",
		VHost:    "/",
	}

	// 生产者选项
	pubOpts := PublisherOptions{
		Exchange:     "test-exchange",
		ExchangeType: "direct",
		RoutingKey:   "test-key",
	}

	// 消费者选项（手动确认）
	consOpts := ConsumerOptions{
		Queue:         "test-queue",
		Exchange:      "test-exchange",
		ExchangeType:  "direct",
		RoutingKey:    "test-key",
		ConsumerTag:   "test-consumer",
		AutoAck:       false,
		Workers:       4,
		PrefetchCount: 4,
	}

	// 创建消费者
	consumer := NewConsumer(
		opts,
		consOpts,
		func(body []byte) error {
			log.Printf("[consumer] recv: %s", body)
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	)
	defer consumer.Close()

	go consumer.Start()

	time.Sleep(500 * time.Millisecond) // 等待消费者启动

	// 创建生产者
	publisher, err := NewPublisher(opts, pubOpts)
	if err != nil {
		t.Fatalf("new publisher failed: %v", err)
	}
	defer publisher.Close()

	// 发送多条消息
	for i := 0; i < 100; i++ {
		msg := fmt.Sprintf("msg-%d", i)
		if err := publisher.Publish([]byte(msg)); err != nil {
			t.Fatalf("publish failed: %v", err)
		}
		time.Sleep(100 * time.Millisecond) // 间隔发送
	}

	// 等待所有消息被消费
	time.Sleep(2 * time.Second)
}
