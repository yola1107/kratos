package gtable

import (
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/gplayer"
)

func (tb *Table) canStart() bool {
	if tb.sitCnt >= 2 {
		return true
	}

	return false
}

func (tb *Table) start() {
}

func (tb *Table) CanEnter(p *gplayer.Player) bool {
	return false
}

func (tb *Table) canExit(p *gplayer.Player) bool {
	return false
}
