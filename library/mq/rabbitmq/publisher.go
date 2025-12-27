package rabbitmq

import (
	"fmt"
	"time"

	"github.com/streadway/amqp"
)

type Publisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	options Options
	pubOpts PublisherOptions
}

// NewPublisher 创建生产者
func NewPublisher(opts Options, pubOpts PublisherOptions) (*Publisher, error) {
	conn, err := amqp.Dial(opts.BuildURL())
	if err != nil {
		return nil, fmt.Errorf("连接RabbitMQ失败: %w", err)
	}

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

	if pubOpts.Exchange != "" {
		if err := ch.ExchangeDeclare(
			pubOpts.Exchange,
			pubOpts.ExchangeType,
			true, false, false, false, nil,
		); err != nil {
			p.Close()
			return nil, err
		}
	}

	return p, nil
}

// Publish 发送消息
func (p *Publisher) Publish(body []byte) error {
	return p.channel.Publish(
		p.pubOpts.Exchange,
		p.pubOpts.RoutingKey,
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

// Close 关闭连接
func (p *Publisher) Close() error {
	var err error
	if p.channel != nil {
		err = p.channel.Close()
	}
	if p.conn != nil {
		if e := p.conn.Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}
