package tlog

import (
	"testing"
)

func TestNewTableLog(t *testing.T) {
	tables := map[int64]*TableLog{}
	for i := 0; i < 1000; i++ {
		tb := NewTableLog(int64(i), true)
		tables[int64(i)] = tb
	}

	tables[0].UserEnter(1001813, 1, 59575)
	tables[5].UserEnter(1001813, 1, 59575)
}
