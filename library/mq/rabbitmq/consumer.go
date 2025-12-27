package rabbitmq

import (
	"log"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

const (
	defaultConnectionTimeout = 30 * time.Second
)

type MessageHandler func([]byte) error

type Consumer struct {
	opts    Options
	copts   ConsumerOptions
	handler MessageHandler
	conn    *amqp.Connection
	ch      *amqp.Channel
	msgs    <-chan amqp.Delivery
	wg      sync.WaitGroup
	quit    chan struct{}
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
	}
}

func (c *Consumer) Start() {
	go func() {
		for {
			select {
			case <-c.quit:
				return
			default:
				if err := c.connectAndConsume(); err != nil {
					log.Printf("not shutdown by self, you can restart: %v\n", err)
					time.Sleep(defaultConnectionTimeout)
				}
			}
		}
	}()
}

func (c *Consumer) connectAndConsume() error {
	conn, err := amqp.Dial(c.opts.BuildURL())
	if err != nil {
		log.Printf("[Consumer] dial failed: %v", err)
		return nil
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		log.Printf("[Consumer] channel failed: %v", err)
		return nil
	}

	if c.copts.Exchange != "" {
		if err := ch.ExchangeDeclare(c.copts.Exchange, c.copts.ExchangeType, true, false, false, false, nil); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			log.Printf("[Consumer] exchange declare failed: %v", err)
			return nil
		}
	}

	if _, err := ch.QueueDeclare(c.copts.Queue, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		log.Printf("[Consumer] queue declare failed: %v", err)
		return nil
	}

	if c.copts.Exchange != "" {
		if err := ch.QueueBind(c.copts.Queue, c.copts.RoutingKey, c.copts.Exchange, false, nil); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			log.Printf("[Consumer] queue bind failed: %v", err)
			return nil
		}
	}

	if err := ch.Qos(c.copts.PrefetchCount, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		log.Printf("[Consumer] qos failed: %v", err)
		return nil
	}

	msgs, err := ch.Consume(c.copts.Queue, c.copts.ConsumerTag, c.copts.AutoAck, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		log.Printf("[Consumer] consume failed: %v", err)
		return nil
	}

	c.conn = conn
	c.ch = ch
	c.msgs = msgs

	var wg sync.WaitGroup
	for i := 0; i < c.copts.Workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c.worker(id)
		}(i)
	}

	notifyClose := make(chan *amqp.Error, 1)
	c.ch.NotifyClose(notifyClose)

	select {
	case <-c.quit:
	case <-notifyClose:
		log.Println("[Consumer] channel closed, will retry...")
	}

	wg.Wait()
	_ = c.ch.Close()
	_ = c.conn.Close()

	return nil
}

func (c *Consumer) worker(id int) {
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

func (c *Consumer) Close() {
	close(c.quit)
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
