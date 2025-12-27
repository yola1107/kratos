package rabbitmq

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

const (
	defaultRetryInterval = 3 * time.Second
)

type MessageHandler func([]byte) error

type Consumer struct {
	opts    Options
	copts   ConsumerOptions
	handler MessageHandler
	mu      sync.Mutex
	conn    *amqp.Connection
	ch      *amqp.Channel
	wg      sync.WaitGroup
	quit    chan struct{}
	stopped chan struct{}
}

func NewConsumer(opts Options, copts ConsumerOptions, handler MessageHandler) *Consumer {
	if copts.Workers <= 0 {
		copts.Workers = 1
	}
	if copts.PrefetchCount <= 0 {
		copts.PrefetchCount = copts.Workers
	}
	if copts.ConsumerTag == "" {
		copts.ConsumerTag = fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	}
	return &Consumer{
		opts:    opts,
		copts:   copts,
		handler: handler,
		quit:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

/*
Start
- 启动一个后台 goroutine
- 自动重连
- Close 后不会再重连
*/
func (c *Consumer) Start() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer close(c.stopped)

		for {
			select {
			case <-c.quit:
				return
			default:
				if err := c.connectAndConsume(); err != nil {
					select {
					case <-c.quit:
						return
					default:
						log.Printf("[Consumer] disconnected, retrying in %s: %v",
							defaultRetryInterval, err)
						time.Sleep(defaultRetryInterval)
					}
				}
			}
		}
	}()
}

func (c *Consumer) connectAndConsume() error {
	conn, err := amqp.Dial(c.opts.BuildURL())
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("channel failed: %w", err)
	}

	// QoS
	if err := ch.Qos(c.copts.PrefetchCount, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	// Exchange
	if c.copts.Exchange != "" {
		if err := ch.ExchangeDeclare(
			c.copts.Exchange,
			c.copts.ExchangeType,
			true, false, false, false, nil,
		); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return err
		}
	}

	// Queue
	if _, err := ch.QueueDeclare(
		c.copts.Queue,
		true, false, false, false, nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	// Bind
	if c.copts.Exchange != "" {
		if err := ch.QueueBind(
			c.copts.Queue,
			c.copts.RoutingKey,
			c.copts.Exchange,
			false,
			nil,
		); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return err
		}
	}

	msgs, err := ch.Consume(
		c.copts.Queue,
		c.copts.ConsumerTag,
		c.copts.AutoAck,
		false, false, false, nil,
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.ch = ch
	c.mu.Unlock()

	// workers
	var workers sync.WaitGroup
	for i := 0; i < c.copts.Workers; i++ {
		workers.Add(1)
		c.wg.Add(1)
		go func(id int) {
			defer workers.Done()
			defer c.wg.Done()
			c.worker(id, msgs)
		}(i)
	}

	notifyClose := make(chan *amqp.Error, 1)
	ch.NotifyClose(notifyClose)

	select {
	case <-c.quit:
		// 主动关闭，不重连
		_ = ch.Close()
		_ = conn.Close()
		workers.Wait()
		return nil

	case err := <-notifyClose:
		// MQ 异常断开，触发重连
		workers.Wait()
		_ = ch.Close()
		_ = conn.Close()
		return err
	}
}

func (c *Consumer) worker(id int, msgs <-chan amqp.Delivery) {
	for {
		select {
		case <-c.quit:
			return
		case d, ok := <-msgs:
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

func (c *Consumer) Close() {
	select {
	case <-c.quit:
		return
	default:
		close(c.quit)
	}

	c.mu.Lock()
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.mu.Unlock()

	c.wg.Wait()
	<-c.stopped
}
