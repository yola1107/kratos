package gtable

import (
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/gplayer"
)

func (tb *Table) OnExitGame(p *gplayer.Player, code int32, msg string) bool {
	return false
}
