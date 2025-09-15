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
	xtime "github.com/yola1107/kratos/v2/transport/tcp/internal/time"
	"github.com/yola1107/kratos/v2/transport/tcp/proto"
	"google.golang.org/grpc/status"
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
		conn *net.TCPConn
		err  error
		r    int
	)
	for {
		tcpLn, ok := lis.(*net.TCPListener)
		if !ok {
			log.Fatal("listener is not a *net.TCPListener")
			return
		}
		if conn, err = tcpLn.AcceptTCP(); err != nil {
			// if listener close then return
			log.Errorf("listener.Accept(\"%s\") error(%v)", lis.Addr().String(), err)
			return
		}
		if err = conn.SetKeepAlive(s.c.TCP.KeepAlive); err != nil {
			log.Errorf("conn.SetKeepAlive() error(%v)", err)
			return
		}
		if err = conn.SetReadBuffer(s.c.TCP.Rcvbuf); err != nil {
			log.Errorf("conn.SetReadBuffer() error(%v)", err)
			return
		}
		if err = conn.SetWriteBuffer(s.c.TCP.Sndbuf); err != nil {
			log.Errorf("conn.SetWriteBuffer() error(%v)", err)
			return
		}
		go s.serveTCP(conn, r)
		if r++; r == maxInt {
			r = 0
		}
	}
}

func (s *Server) serveTCP(conn net.Conn, r int) {
	var (
		// timer
		tr = s.round.Timer(r)
		rp = s.round.Reader(r)
		wp = s.round.Writer(r)
		// ip addr
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// handshake
	uid := uuid.New().String()
	step := 0
	trd = tr.Add(time.Duration(s.c.Protocol.HandshakeTimeout), func() {
		s.disconnectChan <- uid
		conn.Close()
		log.Errorf("key: %s remoteIP: %s step: %d tcp handshake timeout", ch.Key, conn.RemoteAddr().String(), step)
	})
	ch.IP, _, _ = net.SplitHostPort(rAddr)
	// must not setadv, only used in auth
	step = 1
	if p, err = ch.CliProto.Set(); err == nil {
		if err = s.authTCP(conn, rr, p); err == nil {
			err = wr.Flush()
			ch.Key = uid
			hb = time.Duration(s.c.Protocol.HandshakeTimeout)
			b = s.GetBucket(ch.Key)
			b.Put(ch)
			ctx = metadata.NewServerContext(ctx, metadata.New(map[string][]string{
				"remote_ip": {ch.IP},
				"mid":       {ch.Key},
			}))
		}
	}
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
		// log.Infof("ReadTCP. p={op:%d place:%d type:%d seq:%d code:%d body:%+v}", p.Op, p.Place, p.Type, p.Seq, p.Code, p.Body)
		if p.Type == int32(proto.Ping) {
			tr.Set(trd, time.Duration(s.c.Protocol.HeartbeatTimeout))
			p.Body = nil
			step++
		}
		if err = s.Operate(ctx, p); err != nil {
			st, _ := status.FromError(err)
			log.Warnf("Operate err. st.Code=%d(%+v) st.Message=%v", st.Code(), st.Code(), st.Message())
			// break
		}
		ch.CliProto.SetAdv()
		ch.Signal()
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

func (s *Server) dispatchTCP(conn net.Conn, wr *bufio.Writer, wp *bytes.Pool, wb *bytes.Buffer, ch *channel.Channel) {
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
				switch p.Type {
				case int32(proto.Pong):
					if err = p.WriteTCPHeart(wr); err != nil {
						goto failed
					}
				default:
					// log.Infof("DispatchTCP. p={op:%d place:%d type:%d seq:%d code:%d body:%+v}", p.Op, p.Place, p.Type, p.Seq, p.Code, p.Body)
					if err = p.WriteTCP(wr); err != nil {
						goto failed
					}
				}
				// reset payload back to ring
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
	// s.disconnectChan <- ch.Key
	conn.Close()
	wp.Put(wb)
	// must ensure all channel message discard, for reader won't blocking Signal
	for !finish {
		finish = ch.Ready() == proto.ProtoFinish
	}
	return
}

func (s *Server) authTCP(conn net.Conn, rr *bufio.Reader, p *proto.Payload) (err error) {
	if !s.c.Auth.Open {
		return nil
	}
	// reqBody := &proto.Body{}
	// for {
	// 	if err = p.ReadTCP(rr); err != nil {
	// 		return
	// 	}
	// 	if p.Type == int32(proto.Request) {
	// 		if err = gproto.Unmarshal(p.Body, reqBody); err != nil {
	// 			return
	// 		}
	// 		if reqBody.Ops == proto.AuthOps {
	// 			break
	// 		} else {
	// 			log.Errorf("tcp request ops(%d) not auth", reqBody.Ops)
	// 		}
	// 	}
	// }
	// token := string(reqBody.Data[:])
	// ctx := grpcmd.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	// _, err = s.atuhClient.VerifyToken(ctx, &empty.Empty{})
	return
}
