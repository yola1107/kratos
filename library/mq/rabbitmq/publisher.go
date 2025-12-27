package rabbitmq

import (
	"time"

	"github.com/streadway/amqp"
)

type Publisher struct {
	conn    *amqp.Connection
	ch      *amqp.Channel
	pubOpts PublisherOptions
}

func NewPublisher(opts Options, pubOpts PublisherOptions) (*Publisher, error) {
	conn, err := amqp.Dial(opts.BuildURL())
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	if pubOpts.Exchange != "" {
		if err := ch.ExchangeDeclare(
			pubOpts.Exchange,
			pubOpts.ExchangeType,
			true, false, false, false, nil,
		); err != nil {
			ch.Close()
			conn.Close()
			return nil, err
		}
	}

	return &Publisher{
		conn:    conn,
		ch:      ch,
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
	_ = p.ch.Close()
	_ = p.conn.Close()
}
