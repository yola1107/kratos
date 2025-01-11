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
	//size
	_packSize      = 4
	_typeSize      = 1
	_serverIDSize  = 4
	_placeSize     = 4
	_cmdSize       = 2
	_commandSize   = 4
	_reqHeaderSize = _packSize + _typeSize + _serverIDSize + _placeSize + _cmdSize
	_maxPackSize   = int32(_reqHeaderSize) + MaxBodySize
	// offset
	_packOffset     = 0
	_typeOffset     = _packOffset + _packSize
	_serverIDOffset = _typeOffset + _typeSize
	_placeOffset    = _serverIDOffset + _serverIDSize
	_cmdOffset      = _placeOffset + _placeSize
	_commandOffset  = _cmdOffset + _cmdSize
)

var (
	// ErrProtoPackLen proto packet len error
	ErrProtoPackLen = errors.New("default server codec pack length error")
)

var (
	// ProtoReady proto ready
	ProtoReady = &Payload{Op: OpProtoReady}
	// ProtoFinish proto finish
	ProtoFinish = &Payload{Op: OpProtoFinish}
)

func (p *Payload) ReadTCP(rr *bufio.Reader) (err error) {
	var (
		headerLen int
		bodyLen   int
		packLen   int32
		buf       []byte
	)
	headerLen = _reqHeaderSize
	if buf, err = rr.Pop(headerLen); err != nil {
		return
	}
	packLen = int32(binary.LittleEndian.Uint32(buf[_packOffset:_typeOffset]))
	p.Type = int32(buf[_typeOffset])
	p.ServerID = int32(binary.LittleEndian.Uint32(buf[_serverIDOffset:_placeOffset]))
	p.Place = int32(binary.LittleEndian.Uint32(buf[_placeOffset:_cmdOffset]))
	p.Cmd = int32(binary.LittleEndian.Uint16(buf[_cmdOffset:_commandOffset]))
	if p.Cmd == CMD_GAME_DATA {
		if buf, err = rr.Pop(_commandSize); err != nil {
			return
		}
		p.Command = int32(binary.LittleEndian.Uint32(buf[0:_commandSize]))
		headerLen += _commandSize
	} else {
		p.Command = CMD_VOICE_DATA
	}
	if packLen > (int32(headerLen) + MaxBodySize) {
		return ErrProtoPackLen
	}
	if bodyLen = int(packLen - (int32(headerLen) - int32(_packSize))); bodyLen > 0 {
		p.Body, err = rr.Pop(bodyLen)
	} else {
		p.Body = nil
	}
	return
}

func (p *Payload) WriteTCP(wr *bufio.Writer) (err error) {
	var (
		buf       []byte
		packLen   int
		headerLen int
	)
	headerLen = _reqHeaderSize
	if p.Cmd == CMD_GAME_DATA {
		headerLen += _commandSize
	}
	packLen = headerLen - _packSize + len(p.Body)
	if buf, err = wr.Peek(headerLen); err != nil {
		return
	}
	// header
	binary.LittleEndian.PutUint32(buf[_packOffset:], uint32(packLen))
	buf[_typeOffset] = byte(p.Type)
	binary.LittleEndian.PutUint32(buf[_serverIDOffset:], uint32(p.ServerID))
	binary.LittleEndian.PutUint32(buf[_placeOffset:], uint32(p.Place))
	binary.LittleEndian.PutUint16(buf[_cmdOffset:], uint16(p.Cmd))
	if p.Cmd == CMD_GAME_DATA {
		binary.LittleEndian.PutUint32(buf[_commandOffset:], uint32(p.Command))
	}
	if p.Body != nil {
		_, err = wr.Write(p.Body)
	}
	return
}

func (p *Payload) WriteTCPHeart(wr *bufio.Writer) (err error) {
	var (
		buf       []byte
		packLen   int
		headerLen int
	)
	headerLen = _reqHeaderSize + _commandSize
	packLen = headerLen - _packSize
	if buf, err = wr.Peek(headerLen); err != nil {
		return
	}
	// header
	binary.LittleEndian.PutUint32(buf[_packOffset:], uint32(packLen))
	buf[_typeOffset] = byte(p.Type)
	binary.LittleEndian.PutUint32(buf[_serverIDOffset:], uint32(p.ServerID))
	binary.LittleEndian.PutUint32(buf[_placeOffset:], uint32(p.Place))
	binary.LittleEndian.PutUint16(buf[_cmdOffset:], uint16(p.Cmd))
	binary.LittleEndian.PutUint32(buf[_commandOffset:], uint32(p.Command))
	return
}
