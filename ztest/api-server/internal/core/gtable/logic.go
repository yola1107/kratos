package gtable

import (
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/gplayer"
)

func (t *Table) canStart() bool {
	if t.sitCnt >= 2 {
		return true
	}

	return false
}

func (t *Table) start() {
}

func (t *Table) CanEnter(p *gplayer.Player) bool {
	return false
}

func (t *Table) canExit(p *gplayer.Player) bool {
	return false
}
