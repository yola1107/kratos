package rabbitmq

import (
	"fmt"
	"time"

	"github.com/streadway/amqp"
)

// Publisher RabbitMQ生产者
type Publisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	options Options
	pubOpts PublisherOptions
}

// NewPublisher 创建新的生产者
func NewPublisher(opts Options, pubOpts PublisherOptions, options ...Option) (*Publisher, error) {
	// 应用选项
	for _, opt := range options {
		opt(&opts)
	}

	// 连接RabbitMQ
	conn, err := amqp.Dial(opts.BuildURL())
	if err != nil {
		return nil, fmt.Errorf("连接RabbitMQ失败: %w", err)
	}

	// 创建通道
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("创建通道失败: %w", err)
	}

	p := &Publisher{
		conn:    conn,
		channel: ch,
		options: opts,
		pubOpts: pubOpts,
	}

	// 声明交换机（如果指定）
	if pubOpts.ExchangeName != "" {
		if err := p.declareExchange(); err != nil {
			p.Close()
			return nil, err
		}
	}

	return p, nil
}

// declareExchange 声明交换机
func (p *Publisher) declareExchange() error {
	return p.channel.ExchangeDeclare(
		p.pubOpts.ExchangeName,
		p.pubOpts.ExchangeType,
		true,  // 持久化
		false, // 自动删除
		false, // 内部
		false, // 无等待
		nil,   // 参数
	)
}

// Publish 发布消息
func (p *Publisher) Publish(body []byte) error {
	return p.PublishWithRoutingKey(p.pubOpts.RoutingKey, body)
}

// PublishWithRoutingKey 使用指定路由键发布消息
func (p *Publisher) PublishWithRoutingKey(routingKey string, body []byte) error {
	return p.channel.Publish(
		p.pubOpts.ExchangeName,
		routingKey,
		p.pubOpts.Mandatory,
		p.pubOpts.Immediate,
		amqp.Publishing{
			ContentType:  "text/plain",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
}

// PublishWithOptions 使用自定义选项发布消息
func (p *Publisher) PublishWithOptions(routingKey string, body []byte, contentType string) error {
	return p.channel.Publish(
		p.pubOpts.ExchangeName,
		routingKey,
		p.pubOpts.Mandatory,
		p.pubOpts.Immediate,
		amqp.Publishing{
			ContentType:  contentType,
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
}

// Close 关闭连接
func (p *Publisher) Close() error {
	var err error
	if p.channel != nil {
		err = p.channel.Close()
	}
	if p.conn != nil {
		if closeErr := p.conn.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

// IsClosed 检查连接是否已关闭
func (p *Publisher) IsClosed() bool {
	return p.conn == nil || p.conn.IsClosed()
}
