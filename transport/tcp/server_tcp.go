package tcp

import (
	"context"
	"io"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/metadata"
	"github.com/yola1107/kratos/v2/transport/tcp/internal/bucket"
	"github.com/yola1107/kratos/v2/transport/tcp/internal/bufio"
	"github.com/yola1107/kratos/v2/transport/tcp/internal/bytes"
	"github.com/yola1107/kratos/v2/transport/tcp/internal/channel"
	"github.com/yola1107/kratos/v2/transport/tcp/internal/proxy"
	xtime "github.com/yola1107/kratos/v2/transport/tcp/internal/time"
	"github.com/yola1107/kratos/v2/transport/tcp/proto"
)

const (
	maxInt = 1<<31 - 1
)

// StartTCP listen all tcp.bind and start accept connections.
func (s *Server) StartTCP(accept int) (err error) {
	// split N core accept
	for i := 0; i < accept; i++ {
		go s.acceptTCP(s.lis)
	}
	return
}

func (s *Server) acceptTCP(lis net.Listener) {
	var (
		conn net.Conn
		err  error
		r    int
	)
	for {
		if conn, err = lis.Accept(); err != nil {
			// if listener close then return
			log.Errorf("listener.Accept(\"%s\") error(%v)", lis.Addr().String(), err)
			return
		}
		//if err = conn.SetKeepAlive(s.c.TCP.KeepAlive); err != nil {
		//	log.Errorf("conn.SetKeepAlive() error(%v)", err)
		//	return
		//}
		//if err = conn.SetReadBuffer(s.c.TCP.Rcvbuf); err != nil {
		//	log.Errorf("conn.SetReadBuffer() error(%v)", err)
		//	return
		//}
		//if err = conn.SetWriteBuffer(s.c.TCP.Sndbuf); err != nil {
		//	log.Errorf("conn.SetWriteBuffer() error(%v)", err)
		//	return
		//}
		go s.serveTCP(conn, r)
		if r++; r == maxInt {
			r = 0
		}
	}
}

func (s *Server) serveTCP(conn net.Conn, r int) {
	defer func() {
		if err := s.recoveryServer(); err != nil {
			log.Errorf("serceTcp %v", err)
			conn.Close()
		}
	}()
	var (
		tr    = s.round.Timer(r)
		rp    = s.round.Reader(r)
		wp    = s.round.Writer(r)
		lAddr = conn.LocalAddr().String()
		rAddr = conn.RemoteAddr().String()
	)
	log.Infof("start tcp serve \"%s\" with \"%s\"", lAddr, rAddr)
	var (
		err error
		hb  time.Duration
		p   *proto.Payload
		b   *bucket.Bucket
		trd *xtime.TimerData
		rb  = rp.Get()
		wb  = wp.Get()
		ch  = channel.NewChannel(s.c.Protocol.CliProto, s.c.Protocol.SvrProto)
		rr  = &ch.Reader
		wr  = &ch.Writer
	)
	ch.Reader.ResetBuffer(conn, rb.Bytes())
	ch.Writer.ResetBuffer(conn, wb.Bytes())
	//handshake
	uid := uuid.New().String()
	step := 0
	trd = tr.Add(time.Duration(s.c.Protocol.HandshakeTimeout), func() {
		s.disconnectChan <- uid
		conn.Close()
		log.Errorf("key: %s remoteIP: %s step: %d tcp handshake timeout", ch.Key, conn.RemoteAddr().String(), step)
	})
	//proxy
	if s.c.Protocol.Proxy {
		proxyHeader, err := proxy.Read(rr)
		//if err == proxy.ErrNoProxyProtocol {
		//	err = nil
		//}
		if err != nil {
			log.Errorf("proxy err %v", err)
		} else {
			lAddr = proxyHeader.LocalAddr().String()
			rAddr = proxyHeader.RemoteAddr().String()
			log.Infof("proxy tcp serve \"%s\" with \"%s\"", lAddr, rAddr)
		}
	}
	ch.IP, _, _ = net.SplitHostPort(rAddr)
	step = 1
	////auth
	//if s.c.Auth.Open {
	//	if p, err = ch.CliProto.Set(); err == nil {
	//		err = s.authTCP(conn, rr, p)
	//	}
	//}
	ch.Key = uid
	hb = time.Duration(s.c.Protocol.HandshakeTimeout)
	b = s.GetBucket(ch.Key)
	b.Put(ch)
	//md := metadata.MD{
	//	metadata.RemoteIP: ch.IP,
	//	metadata.Mid:      ch.Key,
	//}
	//newCtx := metadata.NewContext(context.Background(), md)
	md := metadata.New()
	md.Set("remote_ip", ch.IP)
	md.Set("mid", ch.Key)
	newCtx := metadata.NewServerContext(context.Background(), md)
	ctx, cancel := context.WithCancel(newCtx)
	defer cancel()

	step = 2
	if err != nil {
		conn.Close()
		rp.Put(rb)
		wp.Put(wb)
		tr.Del(trd)
		log.Errorf("key: %s handshake failed error(%v)", ch.Key, err)
		return
	}
	trd.Key = ch.Key
	tr.Set(trd, hb)
	step = 3
	// hanshake ok start dispatch goroutine
	go s.dispatchTCP(conn, wr, wp, wb, ch)
	for {
		if p, err = ch.CliProto.Set(); err != nil {
			break
		}
		if err = p.ReadTCP(rr); err != nil {
			break
		}
		tr.Set(trd, time.Duration(s.c.Protocol.HeartbeatTimeout))
		if p.Type == int32(proto.Ping) {
			p.Type = int32(proto.Ping)
			p.Body = nil
			//_metricServerReqCodeTotal.Inc("/Ping", "no_user", "0")
			step++
		} else {
			if err = s.Operate(ctx, p); err != nil {
				break
			}
		}
		ch.CliProto.SetAdv()
		ch.Signal()
		//response为空的时候dispatchTCP不处理
		if p.Body != nil || p.Type == int32(proto.Ping) {
			ch.CliProto.SetAdv()
			ch.Signal()
		}
	}
	if err != nil && err != io.EOF && !strings.Contains(err.Error(), "closed") {
		log.Errorf("key: %s server tcp failed error(%v)", ch.Key, err)
	}
	log.Infof("disconnect. key=%s step=%d", ch.Key, step)
	s.disconnectChan <- uid
	b.Del(ch)
	tr.Del(trd)
	rp.Put(rb)
	conn.Close()
	ch.Close()
}

// 请求/响应处理（dispatchTCP）：
//
// 函数dispatchTCP
// 读取来自客户端的消息，处理它们，并发送适当的响应（例如，PING，PO
// 处理来自服务器的响应，并通过 将它们写回客户端bufio.Writer。
// 如果连接关闭或发生错误，它确保正确的
func (s *Server) dispatchTCP(conn net.Conn, wr *bufio.Writer, wp *bytes.Pool, wb *bytes.Buffer, ch *channel.Channel) {
	defer func() {
		if err := s.recoveryServer(); err != nil {
			log.Errorf("dispatchTCP %v", err)
			conn.Close()
		}
	}()
	var (
		err    error
		finish bool
	)
	for {
		var p = ch.Ready()
		switch p {
		case proto.ProtoFinish:
			finish = true
			goto failed
		case proto.ProtoReady:
			// fetch message from svrbox(client send)
			for {
				if p, err = ch.CliProto.Get(); err != nil {
					break
				}
				if p.Type == int32(proto.Ping) {
					p.Type = int32(proto.Pong)
					if err = p.WriteTCPHeart(wr); err != nil {
						goto failed
					}
				} else if p.Type == int32(proto.Response) {
					if err = p.WriteTCP(wr); err != nil {
						goto failed
					}
				}
				p.Body = nil // avoid memory leak
				ch.CliProto.GetAdv()
			}
		default:
			// server send
			if err = p.WriteTCP(wr); err != nil {
				goto failed
			}
		}
		// only hungry flush response
		if err = wr.Flush(); err != nil {
			log.Errorf("Flush error(%v)", err)
			break
		}
	}
failed:
	if err != nil {
		log.Errorf("key: %s dispatch tcp error(%v)", ch.Key, err)
	}
	//s.disconnectChan <- ch.Key
	conn.Close()
	wp.Put(wb)
	// must ensure all channel message discard, for reader won't blocking Signal
	for !finish {
		finish = ch.Ready() == proto.ProtoFinish
	}
	return
}

//func (s *Server) authTCP(conn *net.TCPConn, rr *bufio.Reader, p *proto.Payload) (err error) {
//	reqBody := &proto.Body{}
//	for {
//		if err = p.ReadTCP(rr); err != nil {
//			return
//		}
//		if p.Type == int32(proto.Request) {
//			if err = gproto.Unmarshal(p.Body, reqBody); err != nil {
//				return
//			}
//			if reqBody.Ops == proto.AuthOps {
//				break
//			} else {
//				log.Errorf("tcp request ops(%d) not auth", reqBody.Ops)
//			}
//		}
//	}
//	token := string(reqBody.Data[:])
//	ctx := grpcmd.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
//	_, err = s.atuhClient.VerifyToken(ctx, &empty.Empty{})
//	return
//}
