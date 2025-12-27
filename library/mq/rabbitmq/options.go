package rabbitmq

import (
	"fmt"
	"net/url"
)

// =====================
// Connection Options
// =====================

type Options struct {
	Host     string
	Port     string
	Username string
	Password string
	VHost    string
}

func DefaultOptions() Options {
	return Options{
		Host:     "localhost",
		Port:     "5672",
		Username: "guest",
		Password: "guest",
		VHost:    "/",
	}
}

// BuildURL 构建RabbitMQ连接URL
func (o Options) BuildURL() string {
	return fmt.Sprintf(
		"amqp://%s:%s@%s:%s/%s",
		url.QueryEscape(o.Username),
		url.QueryEscape(o.Password),
		o.Host,
		o.Port,
		url.PathEscape(o.VHost), // 仅对VHost做路径编码
	)
}

// =====================
// Consumer Options
// =====================

type ConsumerOptions struct {
	Queue        string
	Exchange     string
	ExchangeType string
	RoutingKey   string

	ConsumerTag   string
	AutoAck       bool
	PrefetchCount int
}

// 默认消费者选项
func DefaultConsumerOptions() ConsumerOptions {
	return ConsumerOptions{
		Queue:         "default-queue",
		Exchange:      "",
		ExchangeType:  "direct",
		RoutingKey:    "",
		ConsumerTag:   "",
		AutoAck:       false,
		PrefetchCount: 1,
	}
}

// =====================
// Publisher Options
// =====================

type PublisherOptions struct {
	Exchange     string
	ExchangeType string
	RoutingKey   string

	Mandatory bool
	Immediate bool
}

// 默认生产者选项
func DefaultPublisherOptions() PublisherOptions {
	return PublisherOptions{
		Exchange:     "",
		ExchangeType: "direct",
		RoutingKey:   "",
		Mandatory:    false,
		Immediate:    false,
	}
}
