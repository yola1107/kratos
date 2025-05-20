package gtable

import (
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/gplayer"
)

func (t *Table) BroadcastUserEnter(p *gplayer.Player) {}

func (t *Table) SendTableInfo(p *gplayer.Player) {}

func (t *Table) BroadcastUserExit(p *gplayer.Player) {}
