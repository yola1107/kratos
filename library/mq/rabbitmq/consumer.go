package rabbitmq

import (
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

	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
	msgs <-chan amqp.Delivery

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
	return &Consumer{
		opts:    opts,
		copts:   copts,
		handler: handler,
		quit:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

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
						log.Printf("[Consumer] connection error, retrying: %v", err)
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
		log.Printf("[Consumer] dial failed: %v", err)
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		log.Printf("[Consumer] channel failed: %v", err)
		return err
	}

	if c.copts.Exchange != "" {
		if err := ch.ExchangeDeclare(c.copts.Exchange, c.copts.ExchangeType, true, false, false, false, nil); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			log.Printf("[Consumer] exchange declare failed: %v", err)
			return err
		}
	}

	if _, err := ch.QueueDeclare(c.copts.Queue, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		log.Printf("[Consumer] queue declare failed: %v", err)
		return err
	}

	if c.copts.Exchange != "" {
		if err := ch.QueueBind(c.copts.Queue, c.copts.RoutingKey, c.copts.Exchange, false, nil); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			log.Printf("[Consumer] queue bind failed: %v", err)
			return err
		}
	}

	if err := ch.Qos(c.copts.PrefetchCount, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	msgs, err := ch.Consume(c.copts.Queue, c.copts.ConsumerTag, c.copts.AutoAck, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		log.Printf("[Consumer] consume failed: %v", err)
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.ch = ch
	c.msgs = msgs
	c.mu.Unlock()

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < c.copts.Workers; i++ {
		wg.Add(1)
		c.wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer c.wg.Done()
			c.worker(id)
		}(i)
	}

	notifyClose := make(chan *amqp.Error, 1)
	ch.NotifyClose(notifyClose)

	select {
	case <-c.quit:
		// Graceful shutdown
	case err := <-notifyClose:
		if err != nil {
			log.Printf("[Consumer] channel closed: %v", err)
		}
	}

	// Wait for all workers to finish
	wg.Wait()

	// Clean up resources
	c.mu.Lock()
	if c.ch != nil {
		_ = c.ch.Close()
		c.ch = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.msgs = nil
	c.mu.Unlock()

	return nil
}

func (c *Consumer) worker(id int) {
	c.mu.Lock()
	msgs := c.msgs
	c.mu.Unlock()

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
