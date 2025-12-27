package rabbitmq

import (
	"fmt"
	"log"
	"sync"

	"github.com/streadway/amqp"
)

type MessageHandler func([]byte) error

type Consumer struct {
	conn    *amqp.Connection
	ch      *amqp.Channel
	msgs    <-chan amqp.Delivery
	opts    Options
	copts   ConsumerOptions
	handler MessageHandler

	wg   sync.WaitGroup
	quit chan struct{}
}

func NewConsumer(opts Options, copts ConsumerOptions, handler MessageHandler) (*Consumer, error) {
	conn, err := amqp.Dial(opts.BuildURL())
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	if copts.Workers <= 0 {
		copts.Workers = 1
	}
	if copts.PrefetchCount <= 0 {
		copts.PrefetchCount = copts.Workers
	}

	if err := ch.Qos(copts.PrefetchCount, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	if copts.Exchange != "" {
		if err := ch.ExchangeDeclare(
			copts.Exchange,
			copts.ExchangeType,
			true, false, false, false, nil,
		); err != nil {
			ch.Close()
			conn.Close()
			return nil, err
		}
	}

	if _, err := ch.QueueDeclare(
		copts.Queue,
		true, false, false, false, nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	if copts.Exchange != "" {
		if err := ch.QueueBind(
			copts.Queue,
			copts.RoutingKey,
			copts.Exchange,
			false,
			nil,
		); err != nil {
			ch.Close()
			conn.Close()
			return nil, err
		}
	}

	msgs, err := ch.Consume(
		copts.Queue,
		copts.ConsumerTag,
		copts.AutoAck,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &Consumer{
		conn:    conn,
		ch:      ch,
		msgs:    msgs,
		opts:    opts,
		copts:   copts,
		handler: handler,
		quit:    make(chan struct{}),
	}, nil
}

func (c *Consumer) Start() {
	for i := 0; i < c.copts.Workers; i++ {
		c.wg.Add(1)
		go c.worker(i)
	}
}

func (c *Consumer) worker(id int) {
	defer c.wg.Done()

	for {
		select {
		case <-c.quit:
			return

		case d, ok := <-c.msgs:
			if !ok {
				return
			}

			if err := c.handler(d.Body); err != nil {
				log.Printf("[worker-%d] handle error: %v", id, err)
				if !c.copts.AutoAck {
					_ = d.Nack(false, true)
				}
				continue
			}

			if !c.copts.AutoAck {
				_ = d.Ack(false)
			}
		}
	}
}

func (c *Consumer) Stop() {
	close(c.quit)
	c.wg.Wait()
}

func (c *Consumer) Close() {
	c.Stop()
	_ = c.ch.Close()
	_ = c.conn.Close()
}
