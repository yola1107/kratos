package tcp

import (
	"net"
	"time"

	gb "github.com/gogo/protobuf/proto"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/tcp/internal/bufio"
	"github.com/yola1107/kratos/v2/transport/tcp/proto"
)

type MessageHandle func(data []byte)

type ClientConfig struct {
	Addr     string
	Handlers map[int32]MessageHandle
	Token    string
}

type Client struct {
	pushChan  chan *proto.Payload
	closeChan chan bool
	handlers  map[int32]MessageHandle
}

func NewClient(conf *ClientConfig) (c *Client, err error) {
	c = &Client{
		pushChan:  make(chan *proto.Payload, 100),
		closeChan: make(chan bool),
		handlers:  conf.Handlers,
	}
	conn, err := net.Dial("tcp", conf.Addr)
	if err != nil {
		log.Errorf("net.Dial(%s) error(%v)", conf.Addr, err)
		return
	}
	wr := bufio.NewWriter(conn)
	rd := bufio.NewReader(conn)
	go c.handles(conn, rd)
	go c.dispatch(conn, wr)
	if conf.Token != "" {
		c.auth(conf.Token)
	}
	go c.sendHeart()
	go func() {
		for {
			cc := <-c.closeChan
			if cc {
				if err := conn.Close(); err != nil {
					log.Errorf("close err %v", err)
				}
			}
		}
	}()
	return
}

func (c *Client) Request(command int32, msg gb.Message) (err error) {
	var data []byte
	if data, err = gb.Marshal(msg); err != nil {
		return
	}
	p := &proto.Payload{
		Op:       0,
		Type:     int32(proto.NODE_TYPE_GS),
		ServerID: 0,
		Place:    0,
		Cmd:      int32(proto.CMD_GAME_DATA),
		Command:  command,
		Body:     data,
	}
	c.pushChan <- p
	return
}

func (c *Client) auth(token string) (err error) {
	p := &proto.Payload{
		Type:    int32(proto.NODE_TYPE_GS),
		Cmd:     int32(proto.CMD_GAME_DATA),
		Command: int32(proto.AuthReq),
		Body:    []byte(token),
	}
	c.pushChan <- p
	return
}

func (c *Client) Close() {
	c.closeChan <- true
}

func (c *Client) sendHeart() {
	for {
		p := &proto.Payload{
			Type:    int32(proto.NODE_TYPE_GD),
			Cmd:     int32(proto.CMD_GAME_DATA),
			Command: int32(proto.HallPingReq),
		}
		c.pushChan <- p
		time.Sleep(time.Second * 5)
	}
}

func (c *Client) handles(conn net.Conn, rd *bufio.Reader) {
	for {
		p := &proto.Payload{}
		if err := p.ReadTCP(rd); err != nil {
			log.Errorf("ReadTCP err %v", err)
			c.closeChan <- true
			break
		}
		if p.Type == int32(proto.NODE_TYPE_GS) {
			handle, ok := c.handlers[p.Command]
			if !ok {
				log.Errorf("handle func is not exist")
				continue
			}
			handle(p.Body)
		}
	}
}

func (c *Client) dispatch(conn net.Conn, wr *bufio.Writer) {
	for {
		p := <-c.pushChan
		if p.Type == int32(proto.NODE_TYPE_GD) {
			if err := p.WriteTCPHeart(wr); err != nil {
				log.Errorf("WriteTCPHeart err %v", err)
				c.closeChan <- true
				break
			}
		} else {
			if err := p.WriteTCP(wr); err != nil {
				log.Errorf("WriteTCP err %v", err)
				c.closeChan <- true
				break
			}
		}
		if err := wr.Flush(); err != nil {
			log.Errorf("Flush error(%v)", err)
			break
		}
	}
}
