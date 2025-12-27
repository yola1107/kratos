package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"
)

// TestProducerConsumer 测试生产者和消费者
func TestProducerConsumer(t *testing.T) {
	// 配置选项
	opts := Options{
		Host:     "192.168.152.128",
		Port:     "5672",
		Username: "admin",
		Password: "myMQ#@g3359Blue#@test.com",
		VHost:    "/",
	}

	// 生产者选项
	pubOpts := PublisherOptions{
		ExchangeName: "test-exchange",
		ExchangeType: "direct",
		RoutingKey:   "test-key",
		Mandatory:    false,
		Immediate:    false,
	}

	// 消费者选项
	consOpts := ConsumerOptions{
		QueueName:     "test-queue",
		ExchangeName:  "test-exchange",
		ExchangeType:  "direct",
		RoutingKey:    "test-key",
		ConsumerTag:   "test-consumer",
		AutoAck:       false,
		PrefetchCount: 1,
		PrefetchSize:  0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建生产者
	publisher, err := NewPublisher(opts, pubOpts)
	if err != nil {
		log.Fatalf("创建生产者失败: %v", err)
	}
	defer publisher.Close()

	// 创建消费者
	consumer, err := NewConsumer(opts, consOpts, func(body []byte) error {
		log.Printf("[消费者] ✓ 接收: %s", string(body))
		// 模拟处理时间
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	if err != nil {
		log.Fatalf("创建消费者失败: %v", err)
	}
	defer consumer.Close()

	// 启动消费者（goroutine）
	go func() {
		if err := consumer.Start(); err != nil {
			log.Printf("[消费者] 错误: %v", err)
		}
	}()

	// 等待消费者启动
	time.Sleep(500 * time.Millisecond)

	// 启动生产者（goroutine）
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		counter := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				counter++
				message := fmt.Sprintf("消息 #%d - 时间: %s",
					counter, time.Now().Format("2006-01-02 15:04:05.000"))

				if err := publisher.Publish([]byte(message)); err != nil {
					log.Printf("[生产者] 发送失败: %v", err)
					continue
				}

				log.Printf("[生产者] ✓ 发送: %s", message)
			}
		}
	}()

	// 运行30秒
	log.Println("==========================================")
	log.Println("程序运行中，30秒后自动停止...")
	log.Println("==========================================")
	time.Sleep(30 * time.Second)

	log.Println("正在停止...")
	cancel()
	consumer.Stop()

	time.Sleep(1 * time.Second)
	log.Println("程序已停止")
}

// TestSimpleQueue 测试简单队列（不使用交换机）
func TestSimpleQueue(t *testing.T) {
	opts := Options{
		Host:     "192.168.152.128",
		Port:     "5672",
		Username: "admin",
		Password: "myMQ#@g3359Blue#@test.com",
		VHost:    "/",
	}

	// 简单队列生产者（不使用交换机）
	pubOpts := PublisherOptions{
		ExchangeName: "", // 空字符串表示使用默认交换机
		RoutingKey:   "simple-queue",
	}

	// 简单队列消费者
	consOpts := ConsumerOptions{
		QueueName:     "simple-queue",
		ExchangeName:  "", // 不使用交换机
		AutoAck:       false,
		PrefetchCount: 1,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建生产者
	publisher, err := NewPublisher(opts, pubOpts)
	if err != nil {
		log.Fatalf("创建生产者失败: %v", err)
	}
	defer publisher.Close()

	// 创建消费者
	consumer, err := NewConsumer(opts, consOpts, func(body []byte) error {
		log.Printf("[消费者] 接收: %s", string(body))
		return nil
	})
	if err != nil {
		log.Fatalf("创建消费者失败: %v", err)
	}
	defer consumer.Close()

	// 启动消费者
	go func() {
		consumer.Start()
	}()

	time.Sleep(500 * time.Millisecond)

	// 启动生产者
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		counter := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				counter++
				message := fmt.Sprintf("简单消息 #%d", counter)
				publisher.Publish([]byte(message))
			}
		}
	}()

	time.Sleep(10 * time.Second)
	cancel()
	consumer.Stop()
}
