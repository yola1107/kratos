package gtable

import (
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/gplayer"
)

func (tb *Table) BroadcastUserEnter(p *gplayer.Player) {}

func (tb *Table) SendTableInfo(p *gplayer.Player) {}

func (tb *Table) BroadcastUserExit(p *gplayer.Player) {}
