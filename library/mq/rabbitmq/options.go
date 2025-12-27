package rabbitmq

import (
	"fmt"
	"net/url"
)

type Options struct {
	Host     string
	Port     string
	Username string
	Password string
	VHost    string
}

func DefaultOptions() Options {
	return Options{
		Host:     "localhost",
		Port:     "5672",
		Username: "guest",
		Password: "guest",
		VHost:    "/",
	}
}

// BuildURL 构建 amqp:// URI（不玩花活）
func (o Options) BuildURL() string {
	vhost := o.VHost
	if vhost == "/" {
		vhost = ""
	}

	return fmt.Sprintf(
		"amqp://%s:%s@%s:%s/%s",
		url.QueryEscape(o.Username),
		url.QueryEscape(o.Password),
		o.Host,
		o.Port,
		url.PathEscape(vhost),
	)
}

/************ Consumer Options ************/

type ConsumerOptions struct {
	Queue        string
	Exchange     string
	ExchangeType string
	RoutingKey   string

	ConsumerTag   string
	AutoAck       bool
	PrefetchCount int
	Workers       int
}

/************ Publisher Options ************/

type PublisherOptions struct {
	Exchange     string
	ExchangeType string
	RoutingKey   string

	Mandatory bool
	Immediate bool
}
