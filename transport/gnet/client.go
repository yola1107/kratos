package gnet

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	gproto "google.golang.org/protobuf/proto"

	"github.com/yola1107/kratos/v2/middleware"
	"github.com/yola1107/kratos/v2/transport"
	tcpproto "github.com/yola1107/kratos/v2/transport/tcp/proto"
)

// ClientOption configures the gnet client.
type ClientOption func(*clientOptions)

type clientOptions struct {
	endpoint   string
	timeout    time.Duration
	middleware []middleware.Middleware
}

// WithEndpoint sets target endpoint (host:port).
func WithEndpoint(endpoint string) ClientOption {
	return func(o *clientOptions) {
		o.endpoint = endpoint
	}
}

// WithTimeout sets dial/request timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.timeout = timeout
	}
}

// WithMiddleware adds client middleware.
func WithMiddleware(m ...middleware.Middleware) ClientOption {
	return func(o *clientOptions) {
		o.middleware = m
	}
}

// Client is a lightweight gnet client (sync over TCP).
type Client struct {
	opts clientOptions
}

// NewClient creates a gnet client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		opts: clientOptions{
			timeout: 2 * time.Second,
		},
	}
	for _, o := range opts {
		o(&c.opts)
	}
	return c
}

// Invoke sends a request with given ops and decodes response into reply.
func (c *Client) Invoke(ctx context.Context, ops int32, req gproto.Message, reply gproto.Message) error {
	if c.opts.endpoint == "" {
		return fmt.Errorf("gnet client: endpoint is empty")
	}
	bodyData, err := gproto.Marshal(req)
	if err != nil {
		return err
	}
	body := &tcpproto.Body{Ops: ops, Data: bodyData}
	bodyBytes, err := gproto.Marshal(body)
	if err != nil {
		return err
	}
	payload := &tcpproto.Payload{
		Op:   ops,
		Type: int32(tcpproto.Request),
		Seq:  1,
		Body: bodyBytes,
	}

	ctx = transport.NewClientContext(ctx, &Transport{
		endpoint:  c.opts.endpoint,
		operation: fmt.Sprintf("/ops/%d", ops),
		reqHeader: headerCarrier{},
	})

	h := func(ctx context.Context, _ any) (any, error) {
		return nil, c.roundTrip(ctx, payload, reply)
	}
	if len(c.opts.middleware) > 0 {
		h = middleware.Chain(c.opts.middleware...)(h)
	}
	_, err = h(ctx, nil)
	return err
}

func (c *Client) roundTrip(ctx context.Context, payload *tcpproto.Payload, reply gproto.Message) error {
	dialer := &net.Dialer{Timeout: c.opts.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.opts.endpoint)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(c.opts.timeout)); err != nil {
		return err
	}

	out, err := encodePayload(payload)
	if err != nil {
		return err
	}
	if _, err = conn.Write(out); err != nil {
		return err
	}

	resp, err := readPayload(conn)
	if err != nil {
		return err
	}
	if resp.Type != int32(tcpproto.Response) {
		return fmt.Errorf("unexpected payload type: %d", resp.Type)
	}
	if resp.Code != 0 {
		return status.Error(codes.Code(resp.Code), "gnet: server returned error code")
	}
	respBody := &tcpproto.Body{}
	if err := gproto.Unmarshal(resp.Body, respBody); err != nil {
		return err
	}
	if reply == nil {
		return nil
	}
	return gproto.Unmarshal(respBody.Data, reply)
}

func readPayload(r io.Reader) (*tcpproto.Payload, error) {
	var sizeBuf [4]byte
	if _, err := io.ReadFull(r, sizeBuf[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(sizeBuf[:])
	if size == 0 {
		return nil, fmt.Errorf("invalid payload size")
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	p := &tcpproto.Payload{}
	if err := gproto.Unmarshal(buf, p); err != nil {
		return nil, err
	}
	return p, nil
}
