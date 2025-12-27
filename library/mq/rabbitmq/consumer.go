package rabbitmq

import (
	"fmt"
	"log"

	"github.com/streadway/amqp"
)

// MessageHandler 消息处理函数类型
type MessageHandler func([]byte) error

type Consumer struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	options  Options
	consOpts ConsumerOptions
	handler  MessageHandler
	msgs     <-chan amqp.Delivery
}

// NewConsumer 创建消费者
func NewConsumer(opts Options, consOpts ConsumerOptions, handler MessageHandler) (*Consumer, error) {
	conn, err := amqp.Dial(opts.BuildURL())
	if err != nil {
		return nil, fmt.Errorf("连接RabbitMQ失败: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("创建通道失败: %w", err)
	}

	c := &Consumer{
		conn:     conn,
		channel:  ch,
		options:  opts,
		consOpts: consOpts,
		handler:  handler,
	}

	if consOpts.Exchange != "" {
		if err := c.channel.ExchangeDeclare(
			consOpts.Exchange,
			consOpts.ExchangeType,
			true, false, false, false, nil,
		); err != nil {
			c.Close()
			return nil, err
		}
	}

	if _, err := c.channel.QueueDeclare(
		consOpts.Queue,
		true, false, false, false, nil,
	); err != nil {
		c.Close()
		return nil, err
	}

	if consOpts.Exchange != "" {
		if err := c.channel.QueueBind(
			consOpts.Queue,
			consOpts.RoutingKey,
			consOpts.Exchange,
			false,
			nil,
		); err != nil {
			c.Close()
			return nil, err
		}
	}

	if err := c.channel.Qos(consOpts.PrefetchCount, 0, false); err != nil {
		c.Close()
		return nil, err
	}

	msgs, err := c.channel.Consume(
		consOpts.Queue,
		consOpts.ConsumerTag,
		consOpts.AutoAck,
		false, false, false, nil,
	)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.msgs = msgs

	return c, nil
}

// Start 开始消费
func (c *Consumer) Start() error {
	if c.handler == nil {
		return fmt.Errorf("消息处理函数未设置")
	}

	log.Printf("[消费者] 开始消费队列: %s", c.consOpts.Queue)
	for msg := range c.msgs {
		if err := c.handler(msg.Body); err != nil {
			log.Printf("[消费者] 处理消息失败: %v", err)
			if !c.consOpts.AutoAck {
				msg.Nack(false, true)
			}
			continue
		}
		if !c.consOpts.AutoAck {
			if err := msg.Ack(false); err != nil {
				log.Printf("[消费者] 确认消息失败: %v", err)
			}
		}
	}
	return nil
}

// Stop 停止消费
func (c *Consumer) Stop() error {
	if c.channel != nil {
		return c.channel.Cancel(c.consOpts.ConsumerTag, false)
	}
	return nil
}

// Close 关闭连接
func (c *Consumer) Close() error {
	var err error
	if c.channel != nil {
		err = c.channel.Close()
	}
	if c.conn != nil {
		if e := c.conn.Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}
