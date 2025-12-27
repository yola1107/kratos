package rabbitmq

import (
	"fmt"
	"net/url"
)

// Options RabbitMQ连接选项
type Options struct {
	Host     string
	Port     string
	Username string
	Password string
	VHost    string
}

// ConsumerOptions 消费者选项
type ConsumerOptions struct {
	QueueName     string
	ExchangeName  string
	ExchangeType  string
	RoutingKey    string
	ConsumerTag   string
	AutoAck       bool
	PrefetchCount int
	PrefetchSize  int
}

// PublisherOptions 生产者选项
type PublisherOptions struct {
	ExchangeName string
	ExchangeType string
	RoutingKey   string
	Mandatory    bool
	Immediate    bool
}

// DefaultOptions 默认选项
func DefaultOptions() Options {
	return Options{
		Host:     "localhost",
		Port:     "5672",
		Username: "guest",
		Password: "guest",
		VHost:    "/",
	}
}

// DefaultConsumerOptions 默认消费者选项
func DefaultConsumerOptions() ConsumerOptions {
	return ConsumerOptions{
		QueueName:     "default-queue",
		ExchangeName:  "",
		ExchangeType:  "direct",
		RoutingKey:    "",
		ConsumerTag:   "",
		AutoAck:       false,
		PrefetchCount: 1,
		PrefetchSize:  0,
	}
}

// DefaultPublisherOptions 默认生产者选项
func DefaultPublisherOptions() PublisherOptions {
	return PublisherOptions{
		ExchangeName: "",
		ExchangeType: "direct",
		RoutingKey:   "",
		Mandatory:    false,
		Immediate:    false,
	}
}

// BuildURL 构建RabbitMQ连接URL
func (o Options) BuildURL() string {
	encodedUser := url.QueryEscape(o.Username)
	encodedPassword := url.QueryEscape(o.Password)
	encodedVHost := url.QueryEscape(o.VHost)

	return fmt.Sprintf("amqp://%s:%s@%s:%s%s",
		encodedUser, encodedPassword, o.Host, o.Port, encodedVHost)
}

// Option 选项函数类型
type Option func(*Options)

// WithHost 设置主机
func WithHost(host string) Option {
	return func(o *Options) {
		o.Host = host
	}
}

// WithPort 设置端口
func WithPort(port string) Option {
	return func(o *Options) {
		o.Port = port
	}
}

// WithCredentials 设置用户名和密码
func WithCredentials(username, password string) Option {
	return func(o *Options) {
		o.Username = username
		o.Password = password
	}
}

// WithVHost 设置虚拟主机
func WithVHost(vhost string) Option {
	return func(o *Options) {
		o.VHost = vhost
	}
}
