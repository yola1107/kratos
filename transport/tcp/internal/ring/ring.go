package ring

import (
	"github.com/yola1107/kratos/v2/transport/tcp/internal/errors"
	"github.com/yola1107/kratos/v2/transport/tcp/proto"
)

// Ring ring proto buffer.
type Ring struct {
	// read
	rp   uint64
	num  uint64
	mask uint64
	// TODO split cacheline, many cpu cache line size is 64
	// pad [40]byte
	// write
	wp   uint64
	data []proto.Payload
}

// NewRing new a ring buffer.
func NewRing(num int) *Ring {
	r := new(Ring)
	r.init(uint64(num))
	return r
}

// Init init ring.
func (r *Ring) Init(num int) {
	r.init(uint64(num))
}

func (r *Ring) init(num uint64) {
	// 2^N
	if num&(num-1) != 0 {
		for num&(num-1) != 0 {
			num &= (num - 1)
		}
		num = num << 1
	}

	r.num = num
	r.mask = r.num - 1
	r.data = make([]proto.Payload, num)
}

// Get get a proto from ring.
func (r *Ring) Get() (proto *proto.Payload, err error) {
	if r.rp == r.wp {
		return nil, errors.ErrRingEmpty
	}
	proto = &r.data[r.rp&r.mask]
	return
}

// GetAdv incr read index.
func (r *Ring) GetAdv() {
	r.rp++
}

// Set get a proto to write.
func (r *Ring) Set() (proto *proto.Payload, err error) {
	if r.wp-r.rp >= r.num {
		return nil, errors.ErrRingFull
	}
	proto = &r.data[r.wp&r.mask]
	return
}

// SetAdv incr write index.
func (r *Ring) SetAdv() {
	r.wp++
}

// Reset reset ring.
func (r *Ring) Reset() {
	r.rp = 0
	r.wp = 0
	// prevent pad compiler optimization
	// r.pad = [40]byte{}
}
