package file

import (
	"fmt"
	"testing"
)

func TestNewFileLog(t *testing.T) {
	tables := map[int64]*Log{}
	for i := 0; i < 1000; i++ {
		tl := NewFileLog(fmt.Sprintf("table_%d.log", i))
		tables[int64(i)] = tl
	}

	tables[0].userEnter(1, 1001813, 1, 100)
	tables[0].userEnter(1, 1001814, 2, 200)
	tables[5].userEnter(5, 1001815, 1, 520)
}
