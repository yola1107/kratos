package proto

import (
	"encoding/binary"
	"errors"

	"github.com/yola1107/kratos/v2/transport/tcp/internal/bufio"
)

const (
	// MaxBodySize max proto body size
	MaxBodySize = int32(1 << 12)
)

const (
	// size
	_packSize      = 4
	_headerSize    = 2
	_opSize        = 4
	_placeSize     = 4
	_typeSize      = 4
	_seqSize       = 4
	_codeSize      = 4
	_rawHeaderSize = _packSize + _headerSize + _opSize + _placeSize + _typeSize + _seqSize + _codeSize
	_maxPackSize   = MaxBodySize + int32(_rawHeaderSize)
	// offset
	_packOffset   = 0
	_headerOffset = _packOffset + _packSize
	_opOffset     = _headerOffset + _headerSize
	_placeOffset  = _opOffset + _opSize
	_typeOffset   = _placeOffset + _placeSize
	_seqOffset    = _typeOffset + _typeSize
	_codeOffset   = _seqOffset + _seqSize
)

var (
	// ErrProtoPackLen proto packet len error
	ErrProtoPackLen = errors.New("default server codec pack length error")
	// ErrProtoHeaderLen proto header len error
	ErrProtoHeaderLen = errors.New("default server codec header length error")
)

var (
	// ProtoReady proto ready
	ProtoReady = &Payload{Op: OpProtoReady}
	// ProtoFinish proto finish
	ProtoFinish = &Payload{Op: OpProtoFinish}
)

func (p *Payload) ReadTCP(rr *bufio.Reader) (err error) {
	var (
		bodyLen   int
		headerLen int16
		packLen   int32
		buf       []byte
	)
	if buf, err = rr.Pop(_rawHeaderSize); err != nil {
		return
	}
	packLen = int32(binary.LittleEndian.Uint32(buf[_packOffset:_headerOffset])) // [0:4] 包长度=headLen+bodyLen
	headerLen = int16(binary.LittleEndian.Uint16(buf[_headerOffset:_opOffset])) // [4:6]
	p.Op = int32(binary.LittleEndian.Uint32(buf[_opOffset:_placeOffset]))       // [6:10]
	p.Place = int32(binary.LittleEndian.Uint32(buf[_placeOffset:_typeOffset]))  // [10:14]
	p.Type = int32(binary.LittleEndian.Uint32(buf[_typeOffset:_seqOffset]))     // [14:18]
	p.Seq = int32(binary.LittleEndian.Uint32(buf[_seqOffset:_codeOffset]))      // [18:22]
	p.Code = int32(binary.LittleEndian.Uint32(buf[_codeOffset:]))               // [22:]

	if packLen > _maxPackSize {
		return ErrProtoPackLen
	}
	if headerLen != _rawHeaderSize {
		return ErrProtoHeaderLen
	}
	if bodyLen = int(packLen - int32(headerLen)); bodyLen > 0 {
		p.Body, err = rr.Pop(bodyLen)
	} else {
		p.Body = nil
	}
	return
}

func (p *Payload) WriteTCP(wr *bufio.Writer) (err error) {
	var (
		buf     []byte
		packLen int32
	)
	packLen = _rawHeaderSize + int32(len(p.Body))
	if buf, err = wr.Peek(_rawHeaderSize); err != nil {
		return
	}
	// 按顺序写入固定长度字段
	binary.LittleEndian.PutUint32(buf[_packOffset:], uint32(packLen))          // [0:4]
	binary.LittleEndian.PutUint16(buf[_headerOffset:], uint16(_rawHeaderSize)) // [4:6]
	binary.LittleEndian.PutUint32(buf[_opOffset:], uint32(p.Op))               // [6:10]
	binary.LittleEndian.PutUint32(buf[_placeOffset:], uint32(p.Place))         // [10:14]
	binary.LittleEndian.PutUint32(buf[_typeOffset:], uint32(p.Type))           // [14:18]
	binary.LittleEndian.PutUint32(buf[_seqOffset:], uint32(p.Seq))             // [18:22]
	binary.LittleEndian.PutUint32(buf[_codeOffset:], uint32(p.Code))           // [22:]
	if p.Body != nil {
		_, err = wr.Write(p.Body)
	}
	return
}

func (p *Payload) WriteTCPHeart(wr *bufio.Writer) (err error) {
	var (
		buf     []byte
		packLen int32
	)
	packLen = _rawHeaderSize
	if buf, err = wr.Peek(_rawHeaderSize); err != nil {
		return
	}
	binary.LittleEndian.PutUint32(buf[_packOffset:], uint32(packLen))          // [0:4]
	binary.LittleEndian.PutUint16(buf[_headerOffset:], uint16(_rawHeaderSize)) // [4:6]
	binary.LittleEndian.PutUint32(buf[_opOffset:], uint32(p.Op))               // [6:10]
	binary.LittleEndian.PutUint32(buf[_placeOffset:], uint32(p.Place))         // [10:14]
	binary.LittleEndian.PutUint32(buf[_typeOffset:], uint32(p.Type))           // [14:18]
	binary.LittleEndian.PutUint32(buf[_seqOffset:], uint32(p.Seq))             // [18:22]
	binary.LittleEndian.PutUint32(buf[_codeOffset:], uint32(p.Code))           // [22:]
	return
}

// // ReadWebsocket read a proto from websocket connection.
// func (p *Payload) ReadWebsocket(ws *websocket.Conn) (err error) {
//	var (
//		buf []byte
//	)
//	if _, buf, err = ws.ReadMessage(); err != nil {
//		return
//	}
//
//	dataLen := len(buf)
//	if dataLen < (_reqHeaderSize - _heartSeqSize) {
//		return ErrProtoPackLen
//	}
//	p.Place = int32(buf[0])
//	p.Type = int32(buf[_placeSize])
//	seqPos := _placeSize + _typeSize
//	if p.Type == int32(Ping) {
//		p.Seq = int32(buf[seqPos])
//		p.Body = nil
//	} else if p.Type == int32(Push) {
//		p.Body = buf[seqPos:]
//	} else if dataLen > _reqHeaderSize {
//		p.Seq = int32(binary.LittleEndian.Uint16(buf[seqPos:]))
//		pos := seqPos + _seqSize
//		p.Body = buf[pos:]
//	}
//	return
// }
//
// // WriteWebsocket write a proto to websocket connection.
// func (p *Payload) WriteWebsocket(ws *websocket.Conn) (err error) {
//	var (
//		buf        []byte
//		packLen    int
//		headerSize int
//	)
//	if Pattern(p.Type) == Response {
//		headerSize = _placeSize + _typeSize + _seqSize + _codeSize
//	} else {
//		headerSize = _placeSize + _typeSize
//	}
//	packLen = headerSize + len(p.Body)
//	if err = ws.WriteHeader(websocket.BinaryMessage, packLen); err != nil {
//		return
//	}
//	if buf, err = ws.Peek(headerSize); err != nil {
//		return
//	}
//	buf[0] = byte(1)
//	buf[_placeSize] = byte(p.Type)
//	if Pattern(p.Type) == Response {
//		pos := _placeSize + _typeSize
//		binary.LittleEndian.PutUint16(buf[pos:], uint16(p.Seq))
//		pos += _seqSize
//		binary.LittleEndian.PutUint16(buf[pos:], uint16(p.Code))
//	}
//	if p.Body != nil {
//		err = ws.WriteBody(p.Body)
//	}
//	return
// }
//
// // WriteWebsocketHeart write websocket heartbeat with room online.
// func (p *Payload) WriteWebsocketHeart(wr *websocket.Conn) (err error) {
//	var (
//		buf     []byte
//		packLen int
//	)
//	packLen = _placeSize + _typeSize + _heartSeqSize
//	// websocket header
//	if err = wr.WriteHeader(websocket.BinaryMessage, packLen); err != nil {
//		return
//	}
//	if buf, err = wr.Peek(packLen); err != nil {
//		return
//	}
//	// header
//	buf[0] = byte(1)
//	pos := _placeSize
//	buf[pos] = byte(Pong)
//	pos += _typeSize
//	buf[pos] = byte(p.Seq)
//	return
// }
