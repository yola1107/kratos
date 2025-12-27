package rabbitmq

import (
	"time"

	"github.com/streadway/amqp"
)

type Publisher struct {
	opts    Options
	pubOpts PublisherOptions
	conn    *amqp.Connection
	ch      *amqp.Channel
}

func NewPublisher(opts Options, pubOpts PublisherOptions) (*Publisher, error) {
	conn, err := amqp.Dial(opts.BuildURL())
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	if pubOpts.Exchange != "" {
		if err := ch.ExchangeDeclare(
			pubOpts.Exchange,
			pubOpts.ExchangeType,
			true, false, false, false, nil,
		); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, err
		}
	}

	return &Publisher{
		conn:    conn,
		ch:      ch,
		opts:    opts,
		pubOpts: pubOpts,
	}, nil
}

func (p *Publisher) Publish(body []byte) error {
	return p.ch.Publish(
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

func (p *Publisher) Close() {
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
}
