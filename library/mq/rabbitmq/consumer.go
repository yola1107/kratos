package rabbitmq

import (
	"fmt"
	"log"

	"github.com/streadway/amqp"
)

// MessageHandler 消息处理函数类型
type MessageHandler func([]byte) error

// Consumer RabbitMQ消费者
type Consumer struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	options  Options
	consOpts ConsumerOptions
	handler  MessageHandler
	msgs     <-chan amqp.Delivery
}

// NewConsumer 创建新的消费者
func NewConsumer(opts Options, consOpts ConsumerOptions, handler MessageHandler, options ...Option) (*Consumer, error) {
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

	c := &Consumer{
		conn:     conn,
		channel:  ch,
		options:  opts,
		consOpts: consOpts,
		handler:  handler,
	}

	// 声明交换机（如果指定）
	if consOpts.ExchangeName != "" {
		if err := c.declareExchange(); err != nil {
			c.Close()
			return nil, err
		}
	}

	// 声明队列
	if err := c.declareQueue(); err != nil {
		c.Close()
		return nil, err
	}

	// 绑定队列到交换机（如果指定了交换机）
	if consOpts.ExchangeName != "" {
		if err := c.bindQueue(); err != nil {
			c.Close()
			return nil, err
		}
	}

	// 设置QoS
	if err := c.setQoS(); err != nil {
		c.Close()
		return nil, err
	}

	// 开始消费
	if err := c.startConsume(); err != nil {
		c.Close()
		return nil, err
	}

	return c, nil
}

// declareExchange 声明交换机
func (c *Consumer) declareExchange() error {
	return c.channel.ExchangeDeclare(
		c.consOpts.ExchangeName,
		c.consOpts.ExchangeType,
		true,  // 持久化
		false, // 自动删除
		false, // 内部
		false, // 无等待
		nil,   // 参数
	)
}

// declareQueue 声明队列
func (c *Consumer) declareQueue() error {
	_, err := c.channel.QueueDeclare(
		c.consOpts.QueueName,
		true,  // 持久化
		false, // 自动删除
		false, // 排他
		false, // 无等待
		nil,   // 参数
	)
	return err
}

// bindQueue 绑定队列到交换机
func (c *Consumer) bindQueue() error {
	return c.channel.QueueBind(
		c.consOpts.QueueName,
		c.consOpts.RoutingKey,
		c.consOpts.ExchangeName,
		false,
		nil,
	)
}

// setQoS 设置QoS
func (c *Consumer) setQoS() error {
	return c.channel.Qos(
		c.consOpts.PrefetchCount,
		c.consOpts.PrefetchSize,
		false,
	)
}

// startConsume 开始消费
func (c *Consumer) startConsume() error {
	msgs, err := c.channel.Consume(
		c.consOpts.QueueName,
		c.consOpts.ConsumerTag,
		c.consOpts.AutoAck,
		false, // 排他
		false, // 无本地
		false, // 无等待
		nil,   // 参数
	)
	if err != nil {
		return err
	}
	c.msgs = msgs
	return nil
}

// Start 开始处理消息
func (c *Consumer) Start() error {
	if c.handler == nil {
		return fmt.Errorf("消息处理函数未设置")
	}

	log.Printf("[消费者] 开始消费队列: %s", c.consOpts.QueueName)

	for msg := range c.msgs {
		// 处理消息
		if err := c.handler(msg.Body); err != nil {
			log.Printf("[消费者] 处理消息失败: %v", err)
			// 如果处理失败且未自动确认，则拒绝消息
			if !c.consOpts.AutoAck {
				msg.Nack(false, true) // 重新入队
			}
			continue
		}

		// 手动确认消息
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
		if err := c.channel.Cancel(c.consOpts.ConsumerTag, false); err != nil {
			return err
		}
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
		if closeErr := c.conn.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

// IsClosed 检查连接是否已关闭
func (c *Consumer) IsClosed() bool {
	return c.conn == nil || c.conn.IsClosed()
}
