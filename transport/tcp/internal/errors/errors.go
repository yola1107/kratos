package errors

import (
	"errors"
)

// .
var (
	// ring
	ErrRingEmpty = errors.New("ring buffer empty")
	ErrRingFull  = errors.New("ring buffer full")

	// channel
	ErrSignalFullMsgDropped = errors.New("signal channel full, msg dropped")
)
