package gnet

import "github.com/yola1107/kratos/v2/transport"

var _ transport.Transporter = (*Transport)(nil)

// Transport implements transport.Transporter for gnet.
type Transport struct {
	endpoint   string
	operation  string
	reqHeader  headerCarrier
	respHeader headerCarrier
}

// Kind returns the transport kind.
func (t *Transport) Kind() transport.Kind { return transport.KindGNet }

// Endpoint returns the transport endpoint.
func (t *Transport) Endpoint() string { return t.endpoint }

// Operation returns the current operation name.
func (t *Transport) Operation() string { return t.operation }

// RequestHeader returns the request header carrier.
func (t *Transport) RequestHeader() transport.Header { return t.reqHeader }

// ReplyHeader returns the reply header carrier.
func (t *Transport) ReplyHeader() transport.Header { return t.respHeader }

type headerCarrier map[string][]string

func (h headerCarrier) Get(key string) string {
	if vals := h[key]; len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func (h headerCarrier) Set(key string, value string) {
	h[key] = []string{value}
}

func (h headerCarrier) Add(key string, value string) {
	h[key] = append(h[key], value)
}

func (h headerCarrier) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}

func (h headerCarrier) Values(key string) []string {
	return h[key]
}
