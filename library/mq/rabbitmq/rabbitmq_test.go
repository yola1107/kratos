package rabbitmq

import (
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
		PrefetchCount: 1,
	}

	// 创建消费者
	consumer, err := NewConsumer(opts, consOpts, func(body []byte) error {
		log.Printf("[消费者] 接收: %s", string(body))
		time.Sleep(50 * time.Millisecond) // 模拟处理
		return nil
	})
	if err != nil {
		t.Fatalf("创建消费者失败: %v", err)
	}
	defer consumer.Close()
	go consumer.Start()

	time.Sleep(500 * time.Millisecond) // 等待消费者启动

	// 创建生产者
	publisher, err := NewPublisher(opts, pubOpts)
	if err != nil {
		t.Fatalf("创建生产者失败: %v", err)
	}
	defer publisher.Close()

	// 发送多条消息
	for i := 1; i <= 100; i++ {
		msg := []byte("消息 #" + string(rune(i)))
		if err := publisher.Publish(msg); err != nil {
			t.Errorf("发送消息失败: %v", err)
		} else {
			log.Printf("[生产者] 发送: %s", msg)
		}
		time.Sleep(100 * time.Millisecond) // 间隔发送
	}

	// 等待所有消息被消费
	time.Sleep(2 * time.Second)
}
